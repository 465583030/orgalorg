# orgalorg

Ultimate parallel cluster file synchronization tool.

# What

orgalorg provides easy way of synchronizing files acroess cluster.

orgalorg works through ssh & tar, so no unexpected protocol errors will arise.

In default mode of operation (lately referred as sync mode) orgalorg will
perform following steps in order:

1. Acquire global cluster lock (more detailed info above).
2. Create, upload and extract specified files in streaming mode to the
   specified nodes into temporary run directory.
3. Start synchronization tool on each node, that should relocate files from
   temporary run directory to the actual destination.

So, orgalorg expected to work with third-party synchronization tool, that
will do actual files relocation and can be quite intricate, **but orgalorg can
work without that tool and perform simple files sync (more on this later)**.

## Global Cluster Lock

Before doing anything else orgalorg will perform global cluster lock. That lock
is acquired atomically, and no other orgalorg instance can acquire lock if it
is already acquired.

Locking is done via flock'ing specified file or directory on each of target
nodes, and will fail, if flock fails on at least one node.

Directory can be used as lock target as well as ordinary file. `--lock-file`
can be used to specify lock target different from `/`.

After acquiring lock, orgalorg will run heartbeat process, which will check,
that lock is still intact. By default, that check will be performed every 10
seconds. If at least one heartbeat is failed, then orgalorg will abort entire
sync procedure.

User can stop there by using `--lock` or `-L` flag, effectively transform
orgalorg to the distributed locking tool.

## File Upload

Files will be sent from local node to the amount of specified nodes.

orgalorg will perform streaming transfer, so it's safe to synchronize large
files without major memory consumption.

By default, orgalorg will upload files to the temporary run directory. That
behaviour can be changed by using `--root` or `-r` flag. Then, files will be
uploaded to the specified directory.

User can specify `--upload` or `-U` flag to transform orgalorg to the simple
file upload tool. In that mode orgalorg will upload files to the specified
directory and then exit.

orgalorg preserves all file attributes while tranfer as well as user and group
IDs. That behaviour can be changed by using `--no-preserve-uid` and
`--no-preseve-gid` command line options.

By default, orgalorg will keep source file paths as is, creating same directory
layout on the target nodes. E.g., if orgalorg told to upload file `a` while
current working directory is `/b/c/`, orgalorg will upload file to the
`<root>/b/c/a` on the remote nodes. That behaviour can be changed by
specifying `--relative` or `-e` flag. Then, orgalorg will not preserve source
file base directory.

orgalorg will try to upload files under specified user (current user by
default). However, if user has `NOPASSWD` record in the sudoers file on the
remote nodes, `--sudo` or `-x` can be used to elevate to root before uploading
files. It makes possible to login to the remote nodes under normal user and
rewrite system files.

## Synchronization Tool

After file upload orgalorg will execute synchronization tool
(`/usr/lib/orgalorg/sync`). That tool is expected to relocate synced files from
temporary directory to the target directory. However, that tool can perform
arbitrary actions, like reloading system services.

To specify custom synchronization tool user can use `--sync-cmd` or `-n` flag.
Full shell syntax is supported in the argument to that option.

Tool is also expected to communicate with orgalorg using sync protocol
(described below), however, it's not required. If not specified, orgalorg will
communicate with that tool using stdin/stdout streams. User can change that
behaviour using `--simple` or `-m` flag, which will cause orgalorg to treat
specified sync tool as simple shell command. User can even provide stdin
to that program by using `--stdin` or `-i` flag.


# Synchronization Protocol

orgalorg will communicate with given sync tool using special sync protocol,
which gives possibility to perform some actions with synchronization across
entire cluster.

orgalorg will start sync tool as it specified in the command line, without
any modification.

After start, orgalorg will communicate with running sync tool using stdin
and stdout streams. stderr will be passed to user untouched.

All communication messages should be prefixed by special prefix, which is
send by orgalorg in the hello message. All lines on stdout that are not match
given prefix will be printed as is, untouched.

Communcation begins from the hello message.

## Protocol

### HELLO

`orgalorg -> sync tool`

```
<prefix> HELLO
```

Start communication session. All further messages should be prefixed with given
prefix.

### NODE

`orgalorg -> sync tool`

```
<prefix> NODE <node>
```

orgalorg will send node list to the sync tools on each running node.

### START

`orgalorg -> sync tool`

```
<prefix> START
```

Start messages will be sent at the end of the nodes list and means that sync
tool can start doing actions.

### SYNC

`sync tool -> orgalorg`

```
<prefix> SYNC <description>
```

Sync tool can send sync messages after some steps are done to be sure, that
every node in cluster are performing steps gradually, in order.

When orgalorg receives sync message, it will be broadcasted to every connected
sync tool.

### SYNC (broadcasted)

`orgalorg -> sync tool`

```
<prefix> SYNC <node> <description>
```

orgalorg will retransmit incoming sync message from one node to every connected
node (including node, that is sending sync).

Sync tools can wait for specific number of the incoming sync messages to
continue to the next step of execution process.

## Example

`<-` are outgoing messages (from orgalorg to sync tools).

```
<- ORGALORG:132464327653 HELLO
<- ORGALORG:132464327653 NODE [user@node1:22]
<- ORGALORG:132464327653 NODE [user@node2:1234]
<- ORGALORG:132464327653 START
-> (from node1) ORGALORG:132464327653 SYNC phase 1 completed
<- ORGALORG:132464327653 SYNC [user@node1:22] phase 1 completed
-> (from node2) ORGALORG:132464327653 SYNC phase 1 completed
<- ORGALORG:132464327653 SYNC [user@node2:1234] phase 1 completed
```
