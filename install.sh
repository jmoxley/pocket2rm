#!/bin/bash

set -eu

check_go() {
  if ! command -v go &> /dev/null; then
    printf "\nGo could not be found, please install it first\n\n"
    return 1
  fi

  if [ -z "${GOPATH}" ]; then
    export GOPATH=$HOME
  fi

  export GOBIN=$GOPATH/bin
  mkdir -p "$GOBIN"
}

compile_bin_files() {

  printf "\ncompiling pocket2rm...\n"

  cd "$INSTALL_SCRIPT_DIR/cmd/pocket2rm-setup"

  if [ ! -f ./go.mod ]; then
    go mod init pocket2rm-setup
  fi

  go get
  go build main.go

  cd "$INSTALL_SCRIPT_DIR/cmd/pocket2rm"

  if [ ! -f ./go.mod ]; then
    go mod init pocket2rm
  fi

  go get
  GOOS=linux GOARCH=arm GOARM=7 go build -o pocket2rm.arm

  cd "$INSTALL_SCRIPT_DIR/cmd/pocket2rm-reload"

  if [ ! -f ./go.mod ]; then
    go mod init pocket2rm-reload
  fi

  go get
  GOOS=linux GOARCH=arm GOARM=7 go build -o pocket2rm-reload.arm

  printf "pocket2rm successfully compiled"

  printf "\n\n"
  if [ "$1" != "rebuild" ]; then
    "$INSTALL_SCRIPT_DIR/cmd/pocket2rm-setup/main"
  fi
  printf "\n"
}

copy_bin_files_to_remarkable() {
  cd "$INSTALL_SCRIPT_DIR"
  scp "$HOME/.pocket2rm" root@"$REMARKABLE_IP":/home/root/.
  ssh root@"$REMARKABLE_IP" systemctl stop pocket2rm 2> /dev/null;
  ssh root@"$REMARKABLE_IP" systemctl stop pocket2rm-reload 2> /dev/null;
  scp cmd/pocket2rm/pocket2rm.arm root@"$REMARKABLE_IP":/home/root/.
  scp cmd/pocket2rm-reload/pocket2rm-reload.arm root@"$REMARKABLE_IP":/home/root/.
}

copy_service_files_to_remarkable() {
  cd "$INSTALL_SCRIPT_DIR"
  scp cmd/pocket2rm/pocket2rm.service root@"$REMARKABLE_IP":/etc/systemd/system/.
  scp cmd/pocket2rm-reload/pocket2rm-reload.service root@"$REMARKABLE_IP":/etc/systemd/system/.
}

register_and_run_service_on_remarkable() {
  ssh root@"$REMARKABLE_IP" systemctl enable pocket2rm-reload
  ssh root@"$REMARKABLE_IP" systemctl start pocket2rm-reload
}

INSTALL_SCRIPT_DIR=""
REMARKABLE_IP=""

main() {
  INSTALL_SCRIPT_DIR=$(pwd)

  printf "\n"
  read  -r -p "Enter your Remarkable IP address [10.11.99.1]: " REMARKABLE_IP
  REMARKABLE_IP=${REMARKABLE_IP:-10.11.99.1}
  
  if [ ! -f "$HOME/.pocket2rm" ] || [ "$1" == "rebuild" ]; then
    check_go
    compile_bin_files "$1"
    copy_bin_files_to_remarkable
  fi

  copy_service_files_to_remarkable
  register_and_run_service_on_remarkable

  printf "\npocket2rm successfully installed on your Remarkable\n"
}

main "${1-}"
