# CLI Night's Watch

Watch a given folder and anytime a new file is created(moved), it will be uploaded to a generated record based on hourly basis.

The script will look up the local `.RECORD_LOGS` and try to find an existing record that has the following name convention. If nothing is found, it will
create a new record and append the record id to the `.RECORD_LOGS` file

```text
auto-upload-$MM-$DD-$HH
```

Files that have already been uploaded will be saved to the `.UPLOAD_LOGS` with its corresponding hash. When we try to upload the file it will then
look up the `.UPLOAD_LOGS` and skip if it already exists.

## Prerequisite

- Have cocli ready, refer to https://docs.coscene.cn/docs/cli/install
- Have [fswatch](https://github.com/emcrisostomo/fswatch) ready for monitoring file changes

## Usage

```bash
./watch-and-upload.sh -h # for help
./watch-and-upload.sh /PATH/TO/THE/FOLDER # monitor the given folder
./watch-and-upload.sh # monitor the current folder
```

## Improvements

- [ ] Maybe look up for the cloud files for pre-uploaded files
- [ ] Generate a report table
