# Distributed Key Value Store
This project is a Go-based distributed key value store, using Raft and self-made Redis cluster

## Features
- Raft based Leader Election
- Leader Data Catch-up
- Write Ahead Log is yet by list
- Event Dispatcher and Interrupt Channel concept added

## Installation

- 0. ***Currently Only Linux supported, with Docker daemon installed with socket opened for DooD***
> How to open Docker socket for DooD in Linux ?
> 1. open /lib/systemd/system/docker.service 
> 2. 파일의 "ExecStart" 부분에 Open할 주소 명시 후 Reload
> - ExecStart=/usr/bin/dockerd -H fd:// --containerd=/run/containerd/containerd.sock ***-H tcp://0.0.0.0:2375***

- 1. Clone the repository
- 2. "make run"
- 3. To test, run the cli packaged in ./main/cli with ***make cli***
-    Or Directly HTTP request to Server (Document : "http://localhost:8888/api/v1/docs"

## CLI usage
- env CLUSTER_SEVER_URL : 서비스 구동 서버 주소 (default : localhost)
``` 
Usage :
[COMMANDS] [OPTIONS] [OPTIONS]
 
Application Commands : 
get             Retreieve stored value with passed key
set             Store key and value
add             Add new Redis client node (master / slave)
list/ls         Print current registered Redis master, slave clients list
exit/quit       Exit cli
 
get/set Options : 
-k, --key=      key of (key, value) pair to save(set) or retreive(get)
-v, --value=    value of (key, value) pair to save(set)
                                (ex. set -k foo -v bar / get -k foo )
add Options : 
-m, --master=   new Redis node address
                                Used for specifying existing Master client,
                                if 'slave' flag is set)
-s, --slave=    new Redis Slave node address
                                'master' flag must be set to specify new slave's master
```

## Server 
  
- 서버 구성도 :
 <img width="765" alt="스크린샷 2020-02-14 오전 11 13 26" src="https://user-images.githubusercontent.com/48001093/74495405-2d3e7280-4f1b-11ea-9e4d-783e88ca2011.png">

