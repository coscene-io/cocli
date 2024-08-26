#!/usr/bin/env bash

trap 'echo "正在退出脚本..."; exit' INT TERM
set -o errexit
set -o nounset
set -o pipefail
if [[ "${TRACE-0}" == "1" ]]; then
    set -o xtrace
fi

if [[ "${1-}" =~ ^-*h(elp)?$ ]]; then
    echo '用法: ./watch-and-upload.sh [WATCH_DIR]

此脚本定期检查给定目录下的文件，并使用 cocli 上传它们，管理上传记录。

默认的扫描为 20 秒，可以在脚本的配置中设置 SCAN_INTERVAL

WATCH_DIR: 可选。要监控的目录。如果未提供，将使用当前目录。

'
    exit
fi

# 处理 WATCH_DIR 参数
if [ $# -eq 0 ]; then
    WATCH_DIR="$(pwd)"
else
    WATCH_DIR="$1"
fi

# 检查目录是否存在
if [ ! -d "$WATCH_DIR" ]; then
    echo "错误: 目录 '$WATCH_DIR' 不存在或不是一个目录。"
    exit 1
fi

# 定义日志文件
UPLOAD_LOGS="$HOME/.UPLOAD_LOGS"
RECORD_LOGS="$HOME/.RECORD_LOGS"

# 设置延迟时间（秒）
SCAN_INTERVAL=20

get_naming_pattern() {
    echo "auto-upload-$(date +'%Y-%m-%d-%H')"
}

# 确保日志文件存在
touch "$UPLOAD_LOGS" "$RECORD_LOGS"

# 在云端创建新记录并获取 RECORD
create_new_record() {
    local name
    name=$(get_naming_pattern)
    local id
    id=$(cocli record create -t "$name" | awk -F'/' '{print $NF}' | tr -d ' \n' | cut -c 1-36)

    if [ ${#id} -eq 36 ]; then
        printf "%s|%s\n" "$name" "$id" >>"$RECORD_LOGS"
        echo "$id"
    else
        echo "错误: 无法创建有效的记录 ID" >&2
        return 1
    fi
}

# 获取当前小时的记录 ID
get_current_record_id() {
    local current_pattern
    current_pattern=$(get_naming_pattern)
    local id
    id=$(grep "^$current_pattern|" "$RECORD_LOGS" | tail -n 1 | cut -d'|' -f2)

    if [ ${#id} -ne 36 ]; then
        # 尝试从云端获取记录
        id=$(cocli record list | grep "$current_pattern" | awk '{print $1}' | head -n 1)

        if [ ${#id} -eq 36 ]; then
            # 如果从云端找到了有效的ID，将其写入本地记录
            echo "$current_pattern|$id" >>"$RECORD_LOGS"
            echo "从云端找到并缓存了记录: $id" >&2
        else
            # 如果云端也没有找到，创建新记录
            id=$(create_new_record)
        fi
    fi

    echo "$id"
}

# 处理新文件
upload_file() {
    local file="$1"
    file=$(realpath -s "$file")
    [ ! -f "$file" ] && return

    local md5sum
    md5sum=$(md5sum "$file" | cut -d' ' -f1)

    if ! grep -q "$file|$md5sum" "$UPLOAD_LOGS"; then
        local record_id
        record_id=$(get_current_record_id)

        if cocli record upload "$record_id" "$file"; then
            sed -i "\|${file//\//\\/}|d" "$UPLOAD_LOGS"
            echo "$(date +'%Y-%m-%d %H:%M:%S')|$file|$md5sum" >>"$UPLOAD_LOGS"
        else
            echo "上传失败: $file" >&2
        fi
    else
        echo "已跳过 (之前已上传): $file"
    fi
}

function search_files() {
    local dir="$1"

    # 可以使用 find 替代手动递归
    # find "$dir" -type f -name "*.log" -print0 | while IFS= read -r -d '' file; do
    #     upload_file "$file"
    # done

    # 遍历目录中的所有文件和子文件夹
    for file in "$dir"/*; do
        # 跳过当前目录 . 和上级目录 ..
        if [ "$file" == "$dir/." ] || [ "$file" == "$dir/.." ]; then
            continue
        fi

        if [ -d "$file" ]; then
            # 如果是目录，则递归调用 search_files
            search_files "$file"
        elif [ -f "$file" ]; then
            # 如果是文件，检查后缀
            if [[ "$file" == *".log" ]]; then
                upload_file "$file"
            fi
        fi
    done
}

main() {
    while true; do
        echo "开始执行定期扫描..."
        search_files "$WATCH_DIR"

        echo "完成定期扫描，等待上传间隔"
        sleep $SCAN_INTERVAL
    done
}

main "$@"