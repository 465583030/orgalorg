package main

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"github.com/seletskiy/hierr"
)

func startArchiveReceivers(
	lockedNodes *distributedLock,
	rootDir string,
) (*remoteExecution, error) {
	archiveReceiverCommand := []string{
		`tar`, `-x`, `--verbose`, `--directory`,
		rootDir,
	}

	execution, err := runRemoteExecution(lockedNodes, archiveReceiverCommand)
	if err != nil {
		return nil, hierr.Errorf(
			err,
			`can't start tar extraction command: '%v'`,
			archiveReceiverCommand,
		)
	}

	return execution, nil
}

func archiveFilesToWriter(target io.Writer, files []string) error {
	workDir, err := os.Getwd()
	if err != nil {
		return hierr.Errorf(
			err,
			`can't get current working directory`,
		)
	}

	archive := tar.NewWriter(target)
	for fileIndex, fileName := range files {
		logger.Infof(
			"%5d/%d sending file: '%s'",
			fileIndex+1,
			len(files),
			fileName,
		)

		writeFileToArchive(fileName, archive, workDir)
	}

	tracef("closing archive stream, %d files sent", len(files))

	err = archive.Close()
	if err != nil {
		return hierr.Errorf(
			err,
			`can't close tar stream`,
		)
	}

	return nil
}

func writeFileToArchive(
	fileName string,
	archive *tar.Writer,
	workDir string,
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

		Uid: int(fileInfo.Sys().(*syscall.Stat_t).Uid),
		Gid: int(fileInfo.Sys().(*syscall.Stat_t).Gid),

		ModTime: fileInfo.ModTime(),
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
