#!/bin/sh
set -u

# This script is downloading the OS-specific coScene CLI binary with the name - 'cocli', and adds it to PATH

VERSION="latest"
FILE_NAME="cocli"

if [ $# -eq 1 ]; then
    VERSION=$1
    echo "Downloading version $VERSION of coScene CLI..."
else
    echo "Downloading the latest version of coScene CLI..."
fi
echo ""

if uname -s | grep -q -i "darwin"; then
    CLI_OS="darwin"
    if [ "$(uname -m)" = "arm64" ]; then
      ARCH="arm64"
    else
      ARCH="amd64"
    fi
elif uname -s | grep -q -i "linux"; then
    CLI_OS="linux"
    MACHINE_TYPE="$(uname -m)"
    case $MACHINE_TYPE in
        amd64 | x86_64 | x64)
            ARCH="amd64"
            ;;
        aarch64)
            ARCH="arm64"
            ;;
        *)
            echo "Unknown machine type: $MACHINE_TYPE"
            exit 1
            ;;
    esac
else
    echo "Unsupported OS: $(uname -s)"
    exit 1
fi
URL="https://download.coscene.cn/cocli/${VERSION}/${CLI_OS}-${ARCH}.gz"
echo "Downloading from: $URL"
curl -XGET "$URL" > $FILE_NAME.gz
gzip -d $FILE_NAME.gz
chmod +x $FILE_NAME

# Check if the file is downloaded by checking the file size
if [ ! -s $FILE_NAME ]; then
    echo "Failed to download the coScene CLI executable. Please try again."
    exit 1
fi


# Move executable to a destination in path.
# Order is by destination priority.
set -- "/usr/local/bin" "/usr/bin" "/opt/bin"
while [ -n "$1" ]; do
    # Check if destination is in path.
    if echo "$PATH"|grep "$1" -> /dev/null ; then
        if mv $FILE_NAME "$1" ; then
            echo ""
            echo "The $FILE_NAME executable was installed in $1 successfully"
            exit 0
        else
            echo ""
            echo "We'd like to install the coScene CLI executable in $1. Please approve this installation by entering your password."
            if sudo mv $FILE_NAME "$1" ; then
                echo ""
                echo "The $FILE_NAME executable was installed in $1 successfully"
                exit 0
            fi
        fi
    fi
    shift
done

echo "could not find supported destination path in \$PATH"
exit 1
