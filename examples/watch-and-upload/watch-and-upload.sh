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

此脚本使用 fswatch 监控指定目录的新文件，使用 cocli 上传它们，并管理上传记录。

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
NAMING_PATTERN="auto-upload-$(date +'%Y-%m-%d-%H')"

# 确保日志文件存在
touch "$UPLOAD_LOGS" "$RECORD_LOGS"

# 在云端创建新记录并获取 RECORD
create_new_record() {
    local name
    name="$NAMING_PATTERN"
    local id
    id=$(cocli record create -t "$name" | awk -F'/' '{print $NF}' | tr -d ' \n' | cut -c 1-36)

    if [ ${#id} -eq 36 ]; then
        printf "%s|%s\n" "$NAMING_PATTERN" "$id" >>"$RECORD_LOGS"
        echo "$id"
    else
        echo "错误: 无法创建有效的记录 ID" >&2
        return 1
    fi
}

# 获取当前小时的记录 ID
get_current_record_id() {
    local id
    id=$(grep "^$NAMING_PATTERN|" "$RECORD_LOGS" | tail -n 1 | cut -d'|' -f2)

    if [ ${#id} -ne 36 ]; then
        # 尝试从云端获取记录
        id=$(cocli record list | grep "$NAMING_PATTERN" | awk '{print $1}' | head -n 1)

        if [ ${#id} -eq 36 ]; then
            # 如果从云端找到了有效的ID，将其写入本地记录
            echo "$NAMING_PATTERN|$id" >>"$RECORD_LOGS"
            echo "从云端找到并缓存了记录: $id" >&2
        else
            # 如果云端也没有找到，创建新记录
            id=$(create_new_record)
        fi
    fi

    echo "$id"
}

# 处理新文件
process_file() {
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
                process_file "$file"
            fi
        fi
    done
}

# 对于已有文件也做检查
initialize() {
    echo "正在检查现有文件..."
    search_files "$WATCH_DIR"
    echo "初始化完成。"
}

main() {
    echo "开始初始化..."
    initialize

    echo "开始监控目录: $WATCH_DIR"
    fswatch --event Created --event Updated --event MovedTo -0 -r \
        -e "(/|^)\.[^/]*$" \
        -e "/sed.*\.tmp$" \
        "$WATCH_DIR" | while read -d "" event; do

        event=$(realpath -s "$event")
        echo "$event"
        if [ -f "$event" ] && [[ "$(basename "$event")" != .* ]]; then
            echo "正在处理 $event"
            process_file "$event"
        fi
    done
}

main "$@"