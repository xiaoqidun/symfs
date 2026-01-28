// Copyright 2026 肖其顿 (XIAO QI DUN)
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package fs

import (
	"os"
	"path"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/winfsp/cgofuse/fuse"
	"golang.org/x/sys/windows"
)

// FileNotifyInformation 文件通知信息结构体
type FileNotifyInformation struct {
	NextEntryOffset uint32
	Action          uint32
	FileNameLength  uint32
	FileName        [1]uint16
}

// errno 转换错误码
// 入参: err 错误对象
// 返回: int 错误码
func errno(err error) int {
	if err == nil {
		return 0
	}
	if errno, ok := err.(syscall.Errno); ok {
		return -int(errno)
	}
	return -int(fuse.ENOENT)
}

// Init 初始化文件系统
func (s *SymFS) Init() {
	go s.watch()
}

// Destroy 销毁文件系统
func (s *SymFS) Destroy() {
}

// Statfs 获取文件系统统计信息
// 入参: path 路径, stat 统计信息结构体指针
// 返回: int 错误码
func (s *SymFS) Statfs(path string, stat *fuse.Statfs_t) int {
	path = s.realPath(path)
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return errno(err)
	}
	var free, total, avail uint64
	err = windows.GetDiskFreeSpaceEx(pathPtr, &avail, &total, &free)
	if err != nil {
		return errno(err)
	}
	const blockSize = 4096
	stat.Bsize = blockSize
	stat.Frsize = blockSize
	stat.Blocks = total / blockSize
	stat.Bfree = free / blockSize
	stat.Bavail = avail / blockSize
	stat.Namemax = 255
	return 0
}

// Mknod 创建文件节点
// 入参: path 路径, mode 模式, dev 设备号
// 返回: int 错误码
func (s *SymFS) Mknod(path string, mode uint32, dev uint64) int {
	path = s.realPath(path)
	fd, err := syscall.Open(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return errno(err)
	}
	syscall.Close(fd)
	return 0
}

// Mkdir 创建目录
// 入参: path 路径, mode 模式
// 返回: int 错误码
func (s *SymFS) Mkdir(path string, mode uint32) int {
	return errno(os.Mkdir(s.realPath(path), os.FileMode(mode)))
}

// Unlink 删除文件
// 入参: path 路径
// 返回: int 错误码
func (s *SymFS) Unlink(path string) int {
	return errno(os.Remove(s.realPath(path)))
}

// Rmdir 删除目录
// 入参: path 路径
// 返回: int 错误码
func (s *SymFS) Rmdir(path string) int {
	return errno(os.Remove(s.realPath(path)))
}

// Link 创建硬链接
// 入参: oldpath 旧路径, newpath 新路径
// 返回: int 错误码
func (s *SymFS) Link(oldpath string, newpath string) int {
	return errno(os.Link(s.realPath(oldpath), s.realPath(newpath)))
}

// Symlink 创建符号链接
// 入参: target 目标路径, newpath 新路径
// 返回: int 错误码
func (s *SymFS) Symlink(target string, newpath string) int {
	return errno(os.Symlink(target, s.realPath(newpath)))
}

// Readlink 读取符号链接
// 入参: path 路径
// 返回: int 错误码, string 目标路径
func (s *SymFS) Readlink(path string) (int, string) {
	target, err := os.Readlink(s.realPath(path))
	if err != nil {
		return errno(err), ""
	}
	return 0, target
}

// Rename 重命名文件
// 入参: oldpath 旧路径, newpath 新路径
// 返回: int 错误码
func (s *SymFS) Rename(oldpath string, newpath string) int {
	return errno(os.Rename(s.realPath(oldpath), s.realPath(newpath)))
}

// Chmod 修改文件权限
// 入参: path 路径, mode 模式
// 返回: int 错误码
func (s *SymFS) Chmod(path string, mode uint32) int {
	return errno(os.Chmod(s.realPath(path), os.FileMode(mode)))
}

// Chown 修改文件所有者
// 入参: path 路径, uid 用户ID, gid 组ID
// 返回: int 错误码
func (s *SymFS) Chown(path string, uid uint32, gid uint32) int {
	return -int(fuse.ENOSYS)
}

// Utimens 修改文件时间
// 入参: path 路径, tmsp 时间戳数组
// 返回: int 错误码
func (s *SymFS) Utimens(path string, tmsp []fuse.Timespec) int {
	path = s.realPath(path)
	atime := time.Unix(tmsp[0].Sec, tmsp[0].Nsec)
	mtime := time.Unix(tmsp[1].Sec, tmsp[1].Nsec)
	err := os.Chtimes(path, atime, mtime)
	return errno(err)
}

// Access 检查文件访问权限
// 入参: path 路径, mask 掩码
// 返回: int 错误码
func (s *SymFS) Access(path string, mask uint32) int {
	_, err := os.Stat(s.realPath(path))
	return errno(err)
}

// Create 创建并打开文件
// 入参: path 路径, flags 标志位, mode 模式
// 返回: int 错误码, uint64 文件句柄
func (s *SymFS) Create(path string, flags int, mode uint32) (int, uint64) {
	return s.open(path, flags|os.O_CREATE|os.O_TRUNC, mode)
}

// Open 打开文件
// 入参: path 路径, flags 标志位
// 返回: int 错误码, uint64 文件句柄
func (s *SymFS) Open(path string, flags int) (int, uint64) {
	return s.open(path, flags, 0)
}

// Getattr 获取文件属性
// 入参: path 路径, stat 属性结构体指针, fh 文件句柄
// 返回: int 错误码
func (s *SymFS) Getattr(path string, stat *fuse.Stat_t, fh uint64) int {
	path = s.realPath(path)
	fi, err := os.Lstat(path)
	if err != nil {
		return errno(err)
	}
	s.fillStat(stat, fi)
	return 0
}

// Truncate 截断文件
// 入参: path 路径, size 大小, fh 文件句柄
// 返回: int 错误码
func (s *SymFS) Truncate(path string, size int64, fh uint64) int {
	if fh != ^uint64(0) {
		return errno(syscall.Ftruncate(syscall.Handle(fh), size))
	}
	return errno(os.Truncate(s.realPath(path), size))
}

// Read 读取文件内容
// 入参: path 路径, buff 缓冲区, ofst 偏移量, fh 文件句柄
// 返回: int 读取字节数
func (s *SymFS) Read(path string, buff []byte, ofst int64, fh uint64) int {
	h := syscall.Handle(fh)
	var overlapped syscall.Overlapped
	overlapped.Offset = uint32(ofst)
	overlapped.OffsetHigh = uint32(ofst >> 32)
	var n uint32
	err := syscall.ReadFile(h, buff, &n, &overlapped)
	if err != nil && err != syscall.ERROR_HANDLE_EOF {
		return errno(err)
	}
	return int(n)
}

// Write 写入文件内容
// 入参: path 路径, buff 缓冲区, ofst 偏移量, fh 文件句柄
// 返回: int 写入字节数
func (s *SymFS) Write(path string, buff []byte, ofst int64, fh uint64) int {
	h := syscall.Handle(fh)
	var overlapped syscall.Overlapped
	overlapped.Offset = uint32(ofst)
	overlapped.OffsetHigh = uint32(ofst >> 32)
	var n uint32
	err := syscall.WriteFile(h, buff, &n, &overlapped)
	if err != nil {
		return errno(err)
	}
	return int(n)
}

// Flush 刷新文件缓冲
// 入参: path 路径, fh 文件句柄
// 返回: int 错误码
func (s *SymFS) Flush(path string, fh uint64) int {
	syscall.FlushFileBuffers(syscall.Handle(fh))
	return 0
}

// Release 释放文件句柄
// 入参: path 路径, fh 文件句柄
// 返回: int 错误码
func (s *SymFS) Release(path string, fh uint64) int {
	return errno(syscall.CloseHandle(syscall.Handle(fh)))
}

// Fsync 同步文件内容
// 入参: path 路径, datasync 是否仅同步数据, fh 文件句柄
// 返回: int 错误码
func (s *SymFS) Fsync(path string, datasync bool, fh uint64) int {
	err := syscall.FlushFileBuffers(syscall.Handle(fh))
	if err != nil {
		if sysErr, ok := err.(syscall.Errno); ok && sysErr == syscall.ERROR_ACCESS_DENIED {
			return 0
		}
		return errno(err)
	}
	return 0
}

// Opendir 打开目录
// 入参: path 路径
// 返回: int 错误码, uint64 目录句柄
func (s *SymFS) Opendir(path string) (int, uint64) {
	path = s.realPath(path)
	fi, err := os.Stat(path)
	if err != nil {
		return errno(err), ^uint64(0)
	}
	if !fi.IsDir() {
		return -int(fuse.ENOTDIR), ^uint64(0)
	}
	return 0, 0
}

// Readdir 读取目录内容
// 入参: path 路径, fill 填充函数, ofst 偏移量, fh 目录句柄
// 返回: int 错误码
func (s *SymFS) Readdir(path string, fill func(name string, stat *fuse.Stat_t, ofst int64) bool, ofst int64, fh uint64) int {
	path = s.realPath(path)
	f, err := os.Open(path)
	if err != nil {
		return errno(err)
	}
	defer f.Close()
	entries, err := f.Readdir(-1)
	if err != nil {
		return errno(err)
	}
	fill(".", nil, 0)
	fill("..", nil, 0)
	for _, entry := range entries {
		if !fill(entry.Name(), nil, 0) {
			break
		}
	}
	return 0
}

// Releasedir 释放目录句柄
// 入参: path 路径, fh 目录句柄
// 返回: int 错误码
func (s *SymFS) Releasedir(path string, fh uint64) int {
	return 0
}

// Fsyncdir 同步目录内容
// 入参: path 路径, datasync 是否仅同步数据, fh 目录句柄
// 返回: int 错误码
func (s *SymFS) Fsyncdir(path string, datasync bool, fh uint64) int {
	return 0
}

// Setxattr 设置扩展属性
// 入参: path 路径, name 属性名, value 属性值, flags 标志位
// 返回: int 错误码
func (s *SymFS) Setxattr(path string, name string, value []byte, flags int) int {
	return -int(fuse.ENOSYS)
}

// Getxattr 获取扩展属性
// 入参: path 路径, name 属性名
// 返回: int 错误码, []byte 属性值
func (s *SymFS) Getxattr(path string, name string) (int, []byte) {
	return -int(fuse.ENOSYS), nil
}

// Removexattr 删除扩展属性
// 入参: path 路径, name 属性名
// 返回: int 错误码
func (s *SymFS) Removexattr(path string, name string) int {
	return -int(fuse.ENOSYS)
}

// Listxattr 列出扩展属性
// 入参: path 路径, fill 填充函数
// 返回: int 错误码
func (s *SymFS) Listxattr(path string, fill func(name string) bool) int {
	return -int(fuse.ENOSYS)
}

// open 打开文件辅助函数
// 入参: path 路径, flags 标志位, mode 模式
// 返回: int 错误码, uint64 文件句柄
func (s *SymFS) open(path string, flags int, mode uint32) (int, uint64) {
	path = s.realPath(path)
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return errno(err), ^uint64(0)
	}
	var access uint32
	switch flags & (os.O_RDONLY | os.O_WRONLY | os.O_RDWR) {
	case os.O_RDONLY:
		access = windows.GENERIC_READ
	case os.O_WRONLY:
		access = windows.GENERIC_WRITE
	case os.O_RDWR:
		access = windows.GENERIC_READ | windows.GENERIC_WRITE
	}
	if flags&os.O_CREATE != 0 {
		access |= windows.GENERIC_WRITE
	}
	if flags&os.O_APPEND != 0 {
		access &^= windows.GENERIC_WRITE
		access |= windows.FILE_APPEND_DATA
	}
	shareMode := uint32(windows.FILE_SHARE_READ | windows.FILE_SHARE_WRITE | windows.FILE_SHARE_DELETE)
	var createDisposition uint32
	var needTruncate bool
	switch {
	case flags&(os.O_CREATE|os.O_EXCL) == (os.O_CREATE | os.O_EXCL):
		createDisposition = windows.CREATE_NEW
	case flags&(os.O_CREATE|os.O_TRUNC) == (os.O_CREATE | os.O_TRUNC):
		createDisposition = windows.CREATE_ALWAYS
	case flags&os.O_CREATE == os.O_CREATE:
		createDisposition = windows.OPEN_ALWAYS
	case flags&os.O_TRUNC == os.O_TRUNC:
		createDisposition = windows.OPEN_EXISTING
		needTruncate = true
	default:
		createDisposition = windows.OPEN_EXISTING
	}
	attrs := uint32(windows.FILE_ATTRIBUTE_NORMAL)
	h, err := windows.CreateFile(pathPtr, access, shareMode, nil, createDisposition, attrs, 0)
	if err != nil {
		return errno(err), ^uint64(0)
	}
	if needTruncate {
		err := windows.SetEndOfFile(h)
		if err != nil {
			windows.CloseHandle(h)
			h, err = windows.CreateFile(pathPtr, access, shareMode, nil, windows.TRUNCATE_EXISTING, attrs, 0)
			if err != nil {
				return errno(err), ^uint64(0)
			}
		}
	}
	return 0, uint64(h)
}

// watch 监控目录变更
func (s *SymFS) watch() {
	pathPtr, err := windows.UTF16PtrFromString(s.root)
	if err != nil {
		return
	}
	h, err := windows.CreateFile(
		pathPtr,
		windows.FILE_LIST_DIRECTORY,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS,
		0,
	)
	if err != nil {
		return
	}
	defer windows.CloseHandle(h)
	buf := make([]byte, 16384)
	for {
		var bytesReturned uint32
		err = windows.ReadDirectoryChanges(
			h,
			&buf[0],
			uint32(len(buf)),
			true,
			windows.FILE_NOTIFY_CHANGE_FILE_NAME|
				windows.FILE_NOTIFY_CHANGE_DIR_NAME|
				windows.FILE_NOTIFY_CHANGE_ATTRIBUTES|
				windows.FILE_NOTIFY_CHANGE_SIZE|
				windows.FILE_NOTIFY_CHANGE_LAST_WRITE|
				windows.FILE_NOTIFY_CHANGE_CREATION|
				windows.FILE_NOTIFY_CHANGE_SECURITY,
			&bytesReturned,
			nil,
			0,
		)
		if err != nil {
			return
		}
		var offset uint32
		for {
			info := (*FileNotifyInformation)(unsafe.Pointer(&buf[offset]))
			length := info.FileNameLength / 2
			nameSlice := (*[1 << 30]uint16)(unsafe.Pointer(&info.FileName[0]))[:length:length]
			fileName := syscall.UTF16ToString(nameSlice)
			fileName = strings.ReplaceAll(fileName, "\\", "/")
			fullPath := "/" + fileName
			var fuseAction uint32
			switch info.Action {
			case windows.FILE_ACTION_ADDED, windows.FILE_ACTION_RENAMED_NEW_NAME:
				fuseAction = fuse.NOTIFY_CREATE | fuse.NOTIFY_MKDIR
			case windows.FILE_ACTION_REMOVED, windows.FILE_ACTION_RENAMED_OLD_NAME:
				fuseAction = fuse.NOTIFY_UNLINK | fuse.NOTIFY_RMDIR
			case windows.FILE_ACTION_MODIFIED:
				fuseAction = fuse.NOTIFY_CHMOD | fuse.NOTIFY_CHOWN | fuse.NOTIFY_UTIME | fuse.NOTIFY_TRUNCATE
			default:
				fuseAction = fuse.NOTIFY_CREATE | fuse.NOTIFY_UNLINK | fuse.NOTIFY_TRUNCATE
			}
			if info.FileNameLength > 0 {
				s.host.Notify(fullPath, fuseAction)
				dir := path.Dir(fullPath)
				if dir != "/" {
					s.host.Notify(dir, fuseAction)
				}
			}
			if info.NextEntryOffset == 0 {
				break
			}
			offset += info.NextEntryOffset
		}
	}
}

// fillStat 填充统计信息
// 入参: stat 统计信息结构体指针, fi 文件信息接口
func (s *SymFS) fillStat(stat *fuse.Stat_t, fi os.FileInfo) {
	stat.Size = fi.Size()
	stat.Mtim = fuse.NewTimespec(fi.ModTime())
	if sys, ok := fi.Sys().(*syscall.Win32FileAttributeData); ok {
		stat.Atim = fuse.NewTimespec(time.Unix(0, sys.LastAccessTime.Nanoseconds()))
		stat.Birthtim = fuse.NewTimespec(time.Unix(0, sys.CreationTime.Nanoseconds()))
		stat.Ctim = stat.Mtim
	} else {
		stat.Atim = stat.Mtim
		stat.Ctim = stat.Mtim
		stat.Birthtim = stat.Mtim
	}
	mode := uint32(fi.Mode() & os.ModePerm)
	if fi.IsDir() {
		mode |= fuse.S_IFDIR
	} else if fi.Mode()&os.ModeSymlink != 0 {
		mode |= fuse.S_IFLNK
	} else {
		mode |= fuse.S_IFREG
	}
	stat.Mode = mode
	stat.Nlink = 1
	stat.Uid = 0
	stat.Gid = 0
}
