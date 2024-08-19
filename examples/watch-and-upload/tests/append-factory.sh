#!/bin/bash

# 检查参数数量
if [ "$#" -ne 2 ]; then
    echo "用法: $0 <文件名> <时间间隔(秒)>"
    exit 1
fi

FILE_NAME=$1
INTERVAL=$2

# 检查文件是否存在,如果不存在则创建
if [ ! -f "$FILE_NAME" ]; then
    touch "$FILE_NAME"
    echo "创建文件: $FILE_NAME"
fi

# 无限循环,每隔指定时间追加内容
counter=1
while true; do
    echo "追加第 $counter 行内容 - $(date)" >> "$FILE_NAME"
    echo "已追加第 $counter 行内容"
    counter=$((counter + 1))
    sleep "$INTERVAL"
done
