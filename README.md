# coScene CLI (coCLI)

`cocli` 是基于 [cobra](https://github.com/spf13/cobra) 开发的 coScene 命令行工具，用于与 coScene 平台进行交互，便于用户在终端中对 coScene 平台的资源进行管理。

具体的使用方法请参考 [coScene CLI 文档](https://docs.coscene.cn/docs/category/cli)。

另外，`cocli` 所有的命令都可以通过添加 `-h` 参数查看帮助文档。

## 安装

```shell
# 通过 curl 安装
curl -fL https://download.coscene.cn/cocli/install.sh | sh
```

## 本地安装

### 克隆代码

```shell
git clone https://github.com/coscene-io/cocli.git
```
### 本地构建可执行文件

```shell
# 进入项目目录
cd cocli

# 构建可执行文件, 生成的可执行文件在 `./bin` 目录下
make build-binary

# 将可执行文件移动到任意系统路径 PATH 下以便全局使用，当前示例移动到 `/usr/local/bin/` 目录下
mv bin/cocli /usr/local/bin/

# 运行 cocli 命令, 查看帮助文档, 确认安装成功
cocli -h
```
