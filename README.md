# symfs
一个高性能、直通挂载的跨平台 Go 语言文件系统

# 支持平台
目前仅支持 **Windows** 平台，会逐渐支持其他平台

# 安装依赖
项目通过 [cgofuse](https://github.com/winfsp/cgofuse) 实现，需要安装 [winfsp](https://github.com/winfsp/winfsp) 才能使用

# 预编译版
| 操作系统 | 系统架构 | 正常版本 | 后台版本 |
| :--- | :--- | :--- | :--- |
| Windows | 386 | [下载](https://aite.xyz/product/symfs/symfs_windows_386.exe) | [下载](https://aite.xyz/product/symfs/symfs_windows_386_daemon.exe) |
| Windows | AMD64 | [下载](https://aite.xyz/product/symfs/symfs_windows_amd64.exe) | [下载](https://aite.xyz/product/symfs/symfs_windows_amd64_daemon.exe) |
| Windows | ARM64 | [下载](https://aite.xyz/product/symfs/symfs_windows_arm64.exe) | [下载](https://aite.xyz/product/symfs/symfs_windows_arm64_daemon.exe) |

# 使用指南
```powershell
# 基本用法
.\symfs.exe <源目录> <挂载点>

# 使用示例
.\symfs.exe D:\Source D:\Target
```

# 授权协议
本项目使用 [Apache License 2.0](https://github.com/xiaoqidun/symfs/blob/main/LICENSE) 授权协议