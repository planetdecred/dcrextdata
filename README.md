# dcrextdata

[![Build Status](https://img.shields.io/travis/decred/dcrdata.svg)](https://travis-ci.org/raedahgroup/planetdecred)
[![Go Report Card](https://goreportcard.com/badge/github.com/decred/dcrdata)](https://goreportcard.com/report/github.com/planetdecred/dcrextdata)
[![ISC License](https://img.shields.io/badge/license-ISC-blue.svg)](http://copyfree.org)

    dcrextdata is a standalone program for collecting additional info about the decred cryptocurrency like ticker and orderbook data from various exchanges. 

## Requirements
To run **dcrextdata** on your machine you will need the following to be setup.
- `Go` 1.11
- `Postgresql`
- `Nodejs.` Node.js is only used as a build tool, and is not used at runtime
- `Dcrd`

## Setting Up Dcrextdata 
### Step 1. Installations
**Install Go**
* Minimum supported version is 1.11.4. Installation instructions can be found [here](https://golang.org/doc/install).
* Set `$GOPATH` environment variable and add `$GOPATH/bin` to your PATH environment variable as part of the go installation process.

**Install Postgrsql**
* Postgrsql is a relational DBMS used for data storage. Download and installation guide can be found [here](postgresql.org/download)
* *Quick start for  Postgresql*

    If you have a new postgresql install and you want a quick setup for dcrextdata, you can start `postgresql command-line client`(It comes with the installation) with...
    
    ***Linux***
  -  `sudo -u postgres psql` or you could `su` into the postgres user and run `psql` then execute the sql statements below to create a user and database.
    
    ***Windows***
  - Just open the command line interface and type `psql` then execute the sql statements below to create a user and database.
```sql
    CREATE USER {username} WITH PASSWORD '{password}' CREATEDB;
    CREATE DATABASE {databasename} OWNER {username};
```
**Install Nodejs**
* Instructions on how to install `Nodejs` can be found [here](https://nodejs.org/en/download/)

**Install Dcrd**
* Running `dcrd` synchronized to the current best block on the network.
* Download the **decred** release binaries for your operating system from [here](https://github.com/decred/decred-binaries/releases). Check under **Assets**.
* The binary contains other decred packages for connecting to the decred network. 
* Extract **dcrd** Only, [go here](https://docs.decred.org/wallets/cli/cli-installation/) to learn how to setup and run decred binaries.

### Step 2. Getting the source code
- Clone the *dcrextdata* repository. It is conventional to put it under `GOPATH`, but
  this is no longer necessary with go module.

***Linux***
```bash
  git clone https://github.com/planetdecred/dcrextdata $GOPATH/src/github.com/planetdecred/dcrextdata
 ```
 
 ***Windows***
```
  git clone https://github.com/planetdecred/dcrextdata %GOPATH%/src/github.com/planetdecred/dcrextdata
```

### Step 3. Building the source code.
* If you cloned to $GOPATH, set the `GO111MODULE=on` environment variable before building.
Run `export GO111MODULE=on` in terminal (for Mac/Linux) or `setx GO111MODULE on` in command prompt for Windows.
* `cd` to the cloned project directory and run `go build` or `go install`.
Building will place the `dcrextdata` binary in your working directory while install will place the binary in $GOPATH/bin.

#### Building http front-end
* From your project directory, run `npm install` when its done installing packages, 
run `npm run build`.

### Step 4. Configuration
`dcrextdata` can be configured via command-line options or a config file located in the home directory. Start with the sample config file:
```sh
cp sample-dcrextdata.conf ~/.dcrextdata/dcrextdata.conf
```
Then edit `dcrextdata.conf` with your postgres settings. See the output of `dcrextdata --help`
for a list of all options and their default values.

## Running dcrextdata
To run *dcrextdata*, use...
- `dcrextdata` on your command line interface to create database table, fetch data and store the data and launch the http web server. The web server can be disabled by setting `--http=false`
- You can perform a reset by running with the `-R` or `--reset` flag.
- Run `dcrextdata -h` or `dcrextdata help` to get general information of commands and options that can be issued on the cli.
- Use `dcrextdata <command> -h` or   `dcrextdata help <command>` to get detailed information about a command.

## Contributing
See the CONTRIBUTING.md file for details. Here's an overview:

1. Fork this repo to your github account
2. Before starting any work, ensure the master branch of your forked repo is even with this repo's master branch
2. Create a branch for your work (`git checkout -b my-work master`)
3. Write your codes
4. Commit and push to the newly created branch on your forked repo
5. Create a [pull request](https://github.com/planetdecred/dcrextdata/pulls) from your new branch to this repo's master branch
