# 监看指定目录并上传

监看指定目录，每当有新文件或文件改动的时候，上传文件到指定记录

脚本将查看本地的 `$HOME/.RECORD_LOGS` 文件，并尝试查找具有以下命名约定的现有记录。如果没有找到，它将创建一个新记录，并将记录 ID 追加到 `$HOME/.RECORD_LOGS` 文件中。

已上传的文件将与其对应的哈希值一起保存到 `.UPLOAD_LOGS` 中。当我们尝试上传文件时，它会查看 `.UPLOAD_LOGS`，如果文件已存在，则跳过上传。

## 前提条件

- 准备好 cocli，参考 https://docs.coscene.cn/docs/cli/install

## 使用方法

```bash
./watch-and-upload.sh -h # 帮助
./watch-and-upload.sh /PATH/TO/THE/FOLDER # 监看给定的目录
./watch-and-upload.sh # 监看当前目录 $PWD
```
