package main

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/reconquest/go-lineflushwriter"
	"github.com/reconquest/go-prefixwriter"
	"github.com/seletskiy/hierr"
)

func startArchiveReceivers(
	cluster *distributedLock,
	rootDir string,
	sudo bool,
) (*remoteExecution, error) {
	command := []string{}

	prefix := []string{}
	if sudo {
		prefix = sudoCommand
	}

	command = append(command, prefix...)
	command = append(command, `mkdir`, `-p`, rootDir, `&&`)
	command = append(command, prefix...)
	command = append(command, `tar`, `--directory`, rootDir, `-x`)

	if verbose >= verbosityDebug {
		command = append(command, `--verbose`)
	}

	logMutex := &sync.Mutex{}

	runner := &remoteExecutionRunner{command: command}

	execution, err := runner.run(
		cluster,
		func(node *remoteExecutionNode) {
			node.stdout = lineflushwriter.New(
				prefixwriter.New(node.stdout, "{tar} "),
				logMutex,
				true,
			)

			node.stderr = lineflushwriter.New(
				prefixwriter.New(node.stderr, "{tar} "),
				logMutex,
				true,
			)
		},
	)
	if err != nil {
		return nil, hierr.Errorf(
			err,
			`can't start tar extraction command: '%v'`,
			command,
		)
	}

	return execution, nil
}

func archiveFilesToWriter(
	target io.WriteCloser,
	files []string,
	preserveUID, preserveGID bool,
) error {
	workDir, err := os.Getwd()
	if err != nil {
		return hierr.Errorf(
			err,
			`can't get current working directory`,
		)
	}

	archive := tar.NewWriter(target)
	for fileIndex, fileName := range files {
		infof(
			"%5d/%d sending file: '%s'",
			fileIndex+1,
			len(files),
			fileName,
		)

		err = writeFileToArchive(
			fileName,
			archive,
			workDir,
			preserveUID,
			preserveGID,
		)
		if err != nil {
			return hierr.Errorf(
				err,
				`can't write file to archive: '%s'`,
				fileName,
			)
		}
	}

	tracef("closing archive stream, %d files sent", len(files))

	err = archive.Close()
	if err != nil {
		return hierr.Errorf(
			err,
			`can't close tar stream`,
		)
	}

	err = target.Close()
	if err != nil {
		return hierr.Errorf(
			err,
			`can't close target stdin`,
		)
	}

	return nil
}

func writeFileToArchive(
	fileName string,
	archive *tar.Writer,
	workDir string,
	preserveUID, preserveGID bool,
) error {
	fileInfo, err := os.Stat(fileName)

	if err != nil {
		return hierr.Errorf(
			err,
			`can't stat file for archiving: '%s`, fileName,
		)
	}

	// avoid tar warnings about leading slash
	tarFileName := fileName
	if tarFileName[0] == '/' {
		tarFileName = tarFileName[1:]

		fileName, err = filepath.Rel(workDir, fileName)
		if err != nil {
			return hierr.Errorf(
				err,
				`can't make relative path from: '%s'`,
				fileName,
			)
		}
	}

	header := &tar.Header{
		Name: tarFileName,
		Mode: int64(fileInfo.Sys().(*syscall.Stat_t).Mode),
		Size: fileInfo.Size(),

		ModTime: fileInfo.ModTime(),
	}

	if preserveUID {
		header.Uid = int(fileInfo.Sys().(*syscall.Stat_t).Uid)
	}

	if preserveGID {
		header.Gid = int(fileInfo.Sys().(*syscall.Stat_t).Gid)
	}

	tracef(
		hierr.Errorf(
			fmt.Sprintf(
				"size: %d bytes; mode: %o; uid/gid: %d/%d; modtime: %s",
				header.Size,
				header.Mode,
				header.Uid,
				header.Gid,
				header.ModTime,
			),
			`local file: %s; remote file: %s`,
			fileName,
			tarFileName,
		).Error(),
	)

	err = archive.WriteHeader(header)

	if err != nil {
		return hierr.Errorf(
			err,
			`can't write tar header for fileName: '%s'`, fileName,
		)
	}

	fileToArchive, err := os.Open(fileName)
	if err != nil {
		return hierr.Errorf(
			err,
			`can't open fileName for reading: '%s'`,
			fileName,
		)
	}

	_, err = io.Copy(archive, fileToArchive)
	if err != nil {
		return hierr.Errorf(
			err,
			`can't copy file to the archive: '%s'`,
			fileName,
		)
	}

	return nil
}

func getFilesList(relative bool, sources ...string) ([]string, error) {
	files := []string{}

	for _, source := range sources {
		err := filepath.Walk(
			source,
			func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}

				if info.IsDir() {
					return nil
				}

				if !relative {
					path, err = filepath.Abs(path)
					if err != nil {
						return hierr.Errorf(
							err,
							`can't get absolute path for local file: '%s'`,
							path,
						)
					}
				}

				files = append(files, path)

				return nil
			},
		)

		if err != nil {
			return nil, err
		}
	}

	return files, nil
}
