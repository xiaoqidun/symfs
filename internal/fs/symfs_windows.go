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
	"errors"
	"os"
	"path/filepath"
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

const invalidFileHandle = ^uint64(0)

// winErrToFuse Windows 错误码到 FUSE POSIX errno 的映射表
var winErrToFuse = map[syscall.Errno]int{
	windows.ERROR_INVALID_FUNCTION:      int(fuse.EINVAL),
	windows.ERROR_FILE_NOT_FOUND:        int(fuse.ENOENT),
	windows.ERROR_PATH_NOT_FOUND:        int(fuse.ENOENT),
	windows.ERROR_ACCESS_DENIED:         int(fuse.EACCES),
	windows.ERROR_INVALID_HANDLE:        int(fuse.EBADF),
	windows.ERROR_TOO_MANY_OPEN_FILES:   int(fuse.EMFILE),
	windows.ERROR_NOT_ENOUGH_MEMORY:     int(fuse.ENOMEM),
	windows.ERROR_OUTOFMEMORY:           int(fuse.ENOMEM),
	windows.ERROR_INVALID_DRIVE:         int(fuse.ENOENT),
	windows.ERROR_NO_MORE_FILES:         int(fuse.ENOENT),
	windows.ERROR_WRITE_PROTECT:         int(fuse.EROFS),
	windows.ERROR_NOT_READY:             int(fuse.EIO),
	windows.ERROR_SHARING_VIOLATION:     int(fuse.EBUSY),
	windows.ERROR_LOCK_VIOLATION:        int(fuse.EBUSY),
	windows.ERROR_HANDLE_EOF:            int(fuse.EIO),
	windows.ERROR_HANDLE_DISK_FULL:      int(fuse.ENOSPC),
	windows.ERROR_NOT_SUPPORTED:         int(fuse.ENOSYS),
	windows.ERROR_INVALID_PARAMETER:     int(fuse.EINVAL),
	windows.ERROR_INSUFFICIENT_BUFFER:   int(fuse.EINVAL),
	windows.ERROR_INVALID_NAME:          int(fuse.ENOENT),
	windows.ERROR_DIR_NOT_EMPTY:         int(fuse.ENOTEMPTY),
	windows.ERROR_ALREADY_EXISTS:        int(fuse.EEXIST),
	windows.ERROR_FILE_EXISTS:           int(fuse.EEXIST),
	windows.ERROR_BROKEN_PIPE:           int(fuse.EPIPE),
	windows.ERROR_DISK_FULL:             int(fuse.ENOSPC),
	windows.ERROR_CALL_NOT_IMPLEMENTED:  int(fuse.ENOSYS),
	windows.ERROR_NEGATIVE_SEEK:         int(fuse.EINVAL),
	windows.ERROR_SEEK_ON_DEVICE:        int(fuse.ESPIPE),
	windows.ERROR_DIRECTORY:             int(fuse.ENOTDIR),
	windows.ERROR_NOT_EMPTY:             int(fuse.ENOTEMPTY),
	windows.ERROR_NOT_SAME_DEVICE:       int(fuse.EXDEV),
	windows.ERROR_CURRENT_DIRECTORY:     int(fuse.EBUSY),
	windows.ERROR_BAD_NETPATH:           int(fuse.ENOENT),
	windows.ERROR_NETWORK_ACCESS_DENIED: int(fuse.EACCES),
	windows.ERROR_CANNOT_MAKE:           int(fuse.EACCES),
	windows.ERROR_BAD_PATHNAME:          int(fuse.ENOENT),
	windows.ERROR_FILENAME_EXCED_RANGE:  int(fuse.ENAMETOOLONG),
	windows.ERROR_FILE_TOO_LARGE:        int(fuse.EFBIG),
	windows.ERROR_OPERATION_ABORTED:     int(fuse.ECANCELED),
	windows.ERROR_PRIVILEGE_NOT_HELD:    int(fuse.EPERM),
}

// errno 转换错误码
// 入参: err 错误对象
// 返回: int 错误码
func errno(err error) int {
	if err == nil {
		return 0
	}
	var sysErr syscall.Errno
	if errors.As(err, &sysErr) {
		if fuseErr, ok := winErrToFuse[sysErr]; ok {
			return -fuseErr
		}
	}
	return -int(fuse.EIO)
}

// shareMode 获取通用共享模式
// 返回: uint32 Windows 共享模式
func shareMode() uint32 {
	return windows.FILE_SHARE_READ | windows.FILE_SHARE_WRITE | windows.FILE_SHARE_DELETE
}

// openHandle 打开 Windows 句柄
// 入参: path 路径, access 访问权限, disposition 创建模式, attrs 文件属性
// 返回: windows.Handle 句柄, error 错误对象
func openHandle(path string, access uint32, disposition uint32, attrs uint32) (windows.Handle, error) {
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return windows.InvalidHandle, err
	}
	return windows.CreateFile(pathPtr, access, shareMode(), nil, disposition, attrs, 0)
}

// closeHandle 关闭 Windows 句柄
// 入参: fh 文件句柄
// 返回: int 错误码
func closeHandle(fh uint64) int {
	if fh == invalidFileHandle {
		return 0
	}
	return errno(windows.CloseHandle(windows.Handle(fh)))
}

// flushHandle 刷新 Windows 句柄
// 入参: fh 文件句柄
// 返回: int 错误码
func flushHandle(fh uint64) int {
	if fh == invalidFileHandle {
		return 0
	}
	err := windows.FlushFileBuffers(windows.Handle(fh))
	if err != nil {
		if sysErr, ok := err.(syscall.Errno); ok && sysErr == windows.ERROR_ACCESS_DENIED {
			return 0
		}
		return errno(err)
	}
	return 0
}

// timespecToFiletime 转换 FUSE 时间到 Windows 时间
// 入参: tmsp FUSE 时间戳
// 返回: windows.Filetime Windows 时间戳
func timespecToFiletime(tmsp fuse.Timespec) windows.Filetime {
	return windows.NsecToFiletime(time.Unix(tmsp.Sec, tmsp.Nsec).UnixNano())
}

// newOverlapped 创建带偏移的重叠 IO 结构
// 入参: ofst 偏移量
// 返回: windows.Overlapped 重叠 IO 结构, windows.Handle 事件句柄, error 错误对象
func newOverlapped(ofst int64) (windows.Overlapped, windows.Handle, error) {
	event, err := windows.CreateEvent(nil, 1, 0, nil)
	if err != nil {
		return windows.Overlapped{}, 0, err
	}
	overlapped := windows.Overlapped{HEvent: event}
	overlapped.Offset = uint32(ofst)
	overlapped.OffsetHigh = uint32(ofst >> 32)
	return overlapped, event, nil
}

// finishOverlapped 等待重叠 IO 完成
// 入参: h 文件句柄, overlapped 重叠 IO 结构, n 字节数指针, err IO 错误
// 返回: error 错误对象
func finishOverlapped(h windows.Handle, overlapped *windows.Overlapped, n *uint32, err error) error {
	if err == windows.ERROR_IO_PENDING {
		return windows.GetOverlappedResult(h, overlapped, n, true)
	}
	return err
}

// Init 初始化文件系统
func (s *SymFS) Init() {
	s.done = make(chan struct{})
	go s.watch()
}

// Destroy 销毁文件系统
func (s *SymFS) Destroy() {
	if s.done != nil {
		close(s.done)
		s.done = nil
	}
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
	errc, fh := s.open(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if errc != 0 {
		return errc
	}
	return closeHandle(fh)
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
	return errno(windows.Rename(s.realPath(oldpath), s.realPath(newpath)))
}

// Rename3 重命名文件
// 入参: oldpath 旧路径, newpath 新路径, flags 标志位
// 返回: int 错误码
func (s *SymFS) Rename3(oldpath string, newpath string, flags uint32) int {
	if flags&^uint32(fuse.RENAME_NOREPLACE) != 0 {
		return -int(fuse.EINVAL)
	}
	if flags&uint32(fuse.RENAME_NOREPLACE) != 0 {
		_, err := os.Lstat(s.realPath(newpath))
		if err == nil {
			return -int(fuse.EEXIST)
		}
		if !os.IsNotExist(err) {
			return errno(err)
		}
	}
	return s.Rename(oldpath, newpath)
}

// Chmod 修改文件权限
// 入参: path 路径, mode 模式
// 返回: int 错误码
func (s *SymFS) Chmod(path string, mode uint32) int {
	return errno(windows.Chmod(s.realPath(path), mode))
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
	return s.Utimens3(path, tmsp, invalidFileHandle)
}

// Utimens3 修改文件时间
// 入参: path 路径, tmsp 时间戳数组, fh 文件句柄
// 返回: int 错误码
func (s *SymFS) Utimens3(path string, tmsp []fuse.Timespec, fh uint64) int {
	if len(tmsp) < 2 {
		return -int(fuse.EINVAL)
	}
	path = s.realPath(path)
	h := windows.Handle(fh)
	closeAfter := false
	if fh == invalidFileHandle {
		var err error
		h, err = openHandle(path,
			windows.FILE_READ_ATTRIBUTES|windows.FILE_WRITE_ATTRIBUTES,
			windows.OPEN_EXISTING,
			windows.FILE_FLAG_BACKUP_SEMANTICS)
		if err != nil {
			return errno(err)
		}
		closeAfter = true
	}
	if closeAfter {
		defer windows.CloseHandle(h)
	}
	now := fuse.Now()
	var atime *windows.Filetime
	var mtime *windows.Filetime
	var atimeValue windows.Filetime
	var mtimeValue windows.Filetime
	switch tmsp[0].Nsec {
	case fuse.UTIME_OMIT:
	case fuse.UTIME_NOW:
		atimeValue = timespecToFiletime(now)
		atime = &atimeValue
	default:
		atimeValue = timespecToFiletime(tmsp[0])
		atime = &atimeValue
	}
	switch tmsp[1].Nsec {
	case fuse.UTIME_OMIT:
	case fuse.UTIME_NOW:
		mtimeValue = timespecToFiletime(now)
		mtime = &mtimeValue
	default:
		mtimeValue = timespecToFiletime(tmsp[1])
		mtime = &mtimeValue
	}
	return errno(windows.SetFileTime(h, nil, atime, mtime))
}

// Access 检查文件访问权限
// 入参: path 路径, mask 掩码
// 返回: int 错误码
func (s *SymFS) Access(path string, mask uint32) int {
	path = s.realPath(path)
	if mask == fuse.F_OK {
		_, err := os.Lstat(path)
		return errno(err)
	}
	access := uint32(windows.FILE_READ_ATTRIBUTES)
	if mask&fuse.R_OK != 0 {
		access |= windows.FILE_READ_DATA
	}
	if mask&fuse.W_OK != 0 {
		access |= windows.FILE_WRITE_DATA | windows.FILE_APPEND_DATA | windows.FILE_WRITE_ATTRIBUTES
	}
	if mask&fuse.X_OK != 0 {
		access |= windows.FILE_EXECUTE
	}
	if mask&fuse.DELETE_OK != 0 {
		access |= windows.DELETE
	}
	h, err := openHandle(path, access, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS)
	if err != nil {
		return errno(err)
	}
	return errno(windows.CloseHandle(h))
}

// Create 创建并打开文件
// 入参: path 路径, flags 标志位, mode 模式
// 返回: int 错误码, uint64 文件句柄
func (s *SymFS) Create(path string, flags int, mode uint32) (int, uint64) {
	return s.open(path, flags|os.O_CREATE, mode)
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
	if fh != invalidFileHandle {
		var info windows.ByHandleFileInformation
		if err := windows.GetFileInformationByHandle(windows.Handle(fh), &info); err != nil {
			return errno(err)
		}
		s.fillStatFromHandle(stat, &info)
		return 0
	}
	fi, err := os.Lstat(path)
	if err != nil {
		return errno(err)
	}
	s.fillStat(stat, fi)
	h, err := openHandle(path,
		windows.FILE_READ_ATTRIBUTES,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT)
	if err == nil {
		var info windows.ByHandleFileInformation
		if windows.GetFileInformationByHandle(h, &info) == nil {
			s.applyHandleStat(stat, &info)
		}
		windows.CloseHandle(h)
	}
	return 0
}

// Truncate 截断文件
// 入参: path 路径, size 大小, fh 文件句柄
// 返回: int 错误码
func (s *SymFS) Truncate(path string, size int64, fh uint64) int {
	if fh != invalidFileHandle {
		return errno(windows.Ftruncate(windows.Handle(fh), size))
	}
	h, err := openHandle(s.realPath(path),
		windows.FILE_WRITE_DATA|windows.FILE_WRITE_ATTRIBUTES,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL)
	if err != nil {
		return errno(err)
	}
	defer windows.CloseHandle(h)
	return errno(windows.Ftruncate(h, size))
}

// Read 读取文件内容
// 入参: path 路径, buff 缓冲区, ofst 偏移量, fh 文件句柄
// 返回: int 读取字节数
func (s *SymFS) Read(path string, buff []byte, ofst int64, fh uint64) int {
	h := windows.Handle(fh)
	overlapped, event, err := newOverlapped(ofst)
	if err != nil {
		return errno(err)
	}
	defer windows.CloseHandle(event)
	var n uint32
	err = windows.ReadFile(h, buff, &n, &overlapped)
	err = finishOverlapped(h, &overlapped, &n, err)
	if err != nil && err != windows.ERROR_HANDLE_EOF {
		return errno(err)
	}
	return int(n)
}

// Write 写入文件内容
// 入参: path 路径, buff 缓冲区, ofst 偏移量, fh 文件句柄
// 返回: int 写入字节数
func (s *SymFS) Write(path string, buff []byte, ofst int64, fh uint64) int {
	h := windows.Handle(fh)
	overlapped, event, err := newOverlapped(ofst)
	if err != nil {
		return errno(err)
	}
	defer windows.CloseHandle(event)
	var n uint32
	err = windows.WriteFile(h, buff, &n, &overlapped)
	err = finishOverlapped(h, &overlapped, &n, err)
	if err != nil {
		return errno(err)
	}
	return int(n)
}

// Flush 刷新文件缓冲
// 入参: path 路径, fh 文件句柄
// 返回: int 错误码
func (s *SymFS) Flush(path string, fh uint64) int {
	return flushHandle(fh)
}

// Release 释放文件句柄
// 入参: path 路径, fh 文件句柄
// 返回: int 错误码
func (s *SymFS) Release(path string, fh uint64) int {
	return closeHandle(fh)
}

// Fsync 同步文件内容
// 入参: path 路径, datasync 是否仅同步数据, fh 文件句柄
// 返回: int 错误码
func (s *SymFS) Fsync(path string, datasync bool, fh uint64) int {
	return flushHandle(fh)
}

// Opendir 打开目录
// 入参: path 路径
// 返回: int 错误码, uint64 目录句柄
func (s *SymFS) Opendir(path string) (int, uint64) {
	path = s.realPath(path)
	fi, err := os.Lstat(path)
	if err != nil {
		return errno(err), invalidFileHandle
	}
	if !fi.IsDir() {
		return -int(fuse.ENOTDIR), invalidFileHandle
	}
	h, err := openHandle(path,
		windows.FILE_LIST_DIRECTORY|windows.FILE_READ_ATTRIBUTES,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS)
	if err != nil {
		return errno(err), invalidFileHandle
	}
	return 0, uint64(h)
}

// Readdir 读取目录内容
// 入参: path 路径, fill 填充函数, ofst 偏移量, fh 目录句柄
// 返回: int 错误码
func (s *SymFS) Readdir(path string, fill func(name string, stat *fuse.Stat_t, ofst int64) bool, ofst int64, fh uint64) int {
	realPath := s.realPath(path)
	f, err := os.Open(realPath)
	if err != nil {
		return errno(err)
	}
	defer f.Close()
	entries, err := f.Readdir(-1)
	if err != nil {
		return errno(err)
	}
	var dot fuse.Stat_t
	if s.Getattr(path, &dot, invalidFileHandle) == 0 {
		fill(".", &dot, 0)
	} else {
		fill(".", nil, 0)
	}
	fill("..", nil, 0)
	for _, entry := range entries {
		var stat fuse.Stat_t
		s.fillStat(&stat, entry)
		if !fill(entry.Name(), &stat, 0) {
			break
		}
	}
	return 0
}

// Releasedir 释放目录句柄
// 入参: path 路径, fh 目录句柄
// 返回: int 错误码
func (s *SymFS) Releasedir(path string, fh uint64) int {
	return closeHandle(fh)
}

// Fsyncdir 同步目录内容
// 入参: path 路径, datasync 是否仅同步数据, fh 目录句柄
// 返回: int 错误码
func (s *SymFS) Fsyncdir(path string, datasync bool, fh uint64) int {
	return flushHandle(fh)
}

// Chflags 修改文件属性
// 入参: path 路径, flags BSD 标志位
// 返回: int 错误码
func (s *SymFS) Chflags(path string, flags uint32) int {
	path = s.realPath(path)
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return errno(err)
	}
	attrs, err := windows.GetFileAttributes(pathPtr)
	if err != nil {
		return errno(err)
	}
	return errno(windows.SetFileAttributes(pathPtr, flagsToAttributes(attrs, flags)))
}

// Setcrtime 修改文件创建时间
// 入参: path 路径, tmsp 时间戳
// 返回: int 错误码
func (s *SymFS) Setcrtime(path string, tmsp fuse.Timespec) int {
	if tmsp.Nsec == fuse.UTIME_OMIT {
		return 0
	}
	if tmsp.Nsec == fuse.UTIME_NOW {
		tmsp = fuse.Now()
	}
	h, err := openHandle(s.realPath(path),
		windows.FILE_WRITE_ATTRIBUTES,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS)
	if err != nil {
		return errno(err)
	}
	defer windows.CloseHandle(h)
	ctime := timespecToFiletime(tmsp)
	return errno(windows.SetFileTime(h, &ctime, nil, nil))
}

// Setchgtime 修改文件变更时间
// 入参: path 路径, tmsp 时间戳
// 返回: int 错误码
func (s *SymFS) Setchgtime(path string, tmsp fuse.Timespec) int {
	return 0
}

// Getpath 获取真实大小写路径
// 入参: path 路径, fh 文件句柄
// 返回: int 错误码, string 真实路径
func (s *SymFS) Getpath(path string, fh uint64) (int, string) {
	actual, err := s.casePath(path)
	if err != nil {
		return errno(err), ""
	}
	return 0, actual
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
	access := uint32(windows.FILE_READ_ATTRIBUTES | windows.FILE_WRITE_ATTRIBUTES)
	switch flags & (os.O_RDONLY | os.O_WRONLY | os.O_RDWR) {
	case os.O_RDONLY:
		access |= windows.FILE_GENERIC_READ
	case os.O_WRONLY:
		access |= windows.FILE_GENERIC_WRITE
	case os.O_RDWR:
		access |= windows.FILE_GENERIC_READ | windows.FILE_GENERIC_WRITE
	}
	if flags&os.O_CREATE != 0 {
		access |= windows.FILE_GENERIC_WRITE
	}
	if flags&os.O_TRUNC != 0 {
		access |= windows.FILE_WRITE_DATA
	}
	if flags&os.O_APPEND != 0 {
		access &^= windows.FILE_WRITE_DATA
		access |= windows.FILE_APPEND_DATA
	}
	var createDisposition uint32
	switch {
	case flags&(os.O_CREATE|os.O_EXCL) == (os.O_CREATE | os.O_EXCL):
		createDisposition = windows.CREATE_NEW
	case flags&(os.O_CREATE|os.O_TRUNC) == (os.O_CREATE | os.O_TRUNC):
		createDisposition = windows.CREATE_ALWAYS
	case flags&os.O_CREATE == os.O_CREATE:
		createDisposition = windows.OPEN_ALWAYS
	case flags&os.O_TRUNC == os.O_TRUNC:
		createDisposition = windows.TRUNCATE_EXISTING
	default:
		createDisposition = windows.OPEN_EXISTING
	}
	attrs := createAttributes(mode) | windows.FILE_FLAG_OVERLAPPED
	h, err := openHandle(path, access, createDisposition, attrs)
	if err != nil {
		return errno(err), invalidFileHandle
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
		windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OVERLAPPED,
		0,
	)
	if err != nil {
		return
	}
	defer windows.CloseHandle(h)
	event, err := windows.CreateEvent(nil, 0, 0, nil)
	if err != nil {
		return
	}
	defer windows.CloseHandle(event)
	buf := make([]byte, 16384)
	for {
		select {
		case <-s.done:
			return
		default:
		}
		var overlapped windows.Overlapped
		overlapped.HEvent = event
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
			nil,
			(*windows.Overlapped)(unsafe.Pointer(&overlapped)),
			0,
		)
		if err != nil && err != windows.ERROR_IO_PENDING {
			return
		}
		for {
			result, _ := windows.WaitForSingleObject(event, 100)
			if result == uint32(windows.WAIT_OBJECT_0) {
				break
			}
			select {
			case <-s.done:
				windows.CancelIo(h)
				return
			default:
			}
		}
		var bytesReturned uint32
		err = windows.GetOverlappedResult(h, (*windows.Overlapped)(unsafe.Pointer(&overlapped)), &bytesReturned, false)
		if err != nil {
			return
		}
		headerSize := uint32(unsafe.Offsetof(FileNotifyInformation{}.FileName))
		var offset uint32
		for {
			if offset+headerSize > bytesReturned {
				break
			}
			info := (*FileNotifyInformation)(unsafe.Pointer(&buf[offset]))
			if offset+headerSize+info.FileNameLength > bytesReturned {
				break
			}
			length := info.FileNameLength / 2
			nameSlice := (*[1 << 16]uint16)(unsafe.Pointer(&info.FileName[0]))[:length:length]
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
			}
			if info.NextEntryOffset == 0 {
				break
			}
			offset += info.NextEntryOffset
		}
	}
}

// createAttributes 获取创建文件属性
// 入参: mode 文件模式
// 返回: uint32 Windows 文件属性
func createAttributes(mode uint32) uint32 {
	if mode&0222 == 0 {
		return windows.FILE_ATTRIBUTE_READONLY
	}
	return windows.FILE_ATTRIBUTE_NORMAL
}

// attributesToFlags 转换 Windows 属性到 BSD 标志位
// 入参: attrs Windows 文件属性
// 返回: uint32 BSD 标志位
func attributesToFlags(attrs uint32) uint32 {
	var flags uint32
	if attrs&windows.FILE_ATTRIBUTE_HIDDEN != 0 {
		flags |= fuse.UF_HIDDEN
	}
	if attrs&windows.FILE_ATTRIBUTE_READONLY != 0 {
		flags |= fuse.UF_READONLY
	}
	if attrs&windows.FILE_ATTRIBUTE_SYSTEM != 0 {
		flags |= fuse.UF_SYSTEM
	}
	if attrs&windows.FILE_ATTRIBUTE_ARCHIVE != 0 {
		flags |= fuse.UF_ARCHIVE
	}
	return flags
}

// flagsToAttributes 转换 BSD 标志位到 Windows 属性
// 入参: attrs 原 Windows 文件属性, flags BSD 标志位
// 返回: uint32 新 Windows 文件属性
func flagsToAttributes(attrs uint32, flags uint32) uint32 {
	attrs &^= windows.FILE_ATTRIBUTE_NORMAL |
		windows.FILE_ATTRIBUTE_HIDDEN |
		windows.FILE_ATTRIBUTE_READONLY |
		windows.FILE_ATTRIBUTE_SYSTEM |
		windows.FILE_ATTRIBUTE_ARCHIVE
	if flags&fuse.UF_HIDDEN != 0 {
		attrs |= windows.FILE_ATTRIBUTE_HIDDEN
	}
	if flags&fuse.UF_READONLY != 0 {
		attrs |= windows.FILE_ATTRIBUTE_READONLY
	}
	if flags&fuse.UF_SYSTEM != 0 {
		attrs |= windows.FILE_ATTRIBUTE_SYSTEM
	}
	if flags&fuse.UF_ARCHIVE != 0 {
		attrs |= windows.FILE_ATTRIBUTE_ARCHIVE
	}
	if attrs == 0 {
		attrs |= windows.FILE_ATTRIBUTE_NORMAL
	}
	return attrs
}

// applyReadonlyMode 应用只读属性到模式
// 入参: stat 统计信息结构体指针, attrs Windows 文件属性
func applyReadonlyMode(stat *fuse.Stat_t, attrs uint32) {
	if attrs&windows.FILE_ATTRIBUTE_READONLY != 0 {
		stat.Mode &^= fuse.S_IWUSR | fuse.S_IWGRP | fuse.S_IWOTH
		return
	}
	if stat.Mode&fuse.S_IFMT != fuse.S_IFLNK {
		stat.Mode |= fuse.S_IWUSR | fuse.S_IWGRP | fuse.S_IWOTH
	}
}

// filetimeToTimespec 转换 Windows 时间到 FUSE 时间
// 入参: ft Windows 时间戳
// 返回: fuse.Timespec FUSE 时间戳
func filetimeToTimespec(ft windows.Filetime) fuse.Timespec {
	return fuse.NewTimespec(time.Unix(0, ft.Nanoseconds()))
}

// setBlocks 填充块信息
// 入参: stat 统计信息结构体指针
func setBlocks(stat *fuse.Stat_t) {
	const blockSize = 4096
	stat.Blksize = blockSize
	stat.Blocks = (stat.Size + 511) / 512
}

// fillStatFromHandle 通过句柄信息填充统计信息
// 入参: stat 统计信息结构体指针, info Windows 句柄信息
func (s *SymFS) fillStatFromHandle(stat *fuse.Stat_t, info *windows.ByHandleFileInformation) {
	*stat = fuse.Stat_t{}
	mode := uint32(0666)
	if info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0 {
		mode = 0777 | fuse.S_IFDIR
	} else {
		mode |= fuse.S_IFREG
	}
	stat.Mode = mode
	s.applyHandleStat(stat, info)
}

// applyHandleStat 应用 Windows 句柄统计信息
// 入参: stat 统计信息结构体指针, info Windows 句柄信息
func (s *SymFS) applyHandleStat(stat *fuse.Stat_t, info *windows.ByHandleFileInformation) {
	size := int64(uint64(info.FileSizeHigh)<<32 | uint64(info.FileSizeLow))
	if stat.Mode&fuse.S_IFMT != fuse.S_IFLNK {
		stat.Size = size
	}
	stat.Atim = filetimeToTimespec(info.LastAccessTime)
	stat.Mtim = filetimeToTimespec(info.LastWriteTime)
	stat.Ctim = stat.Mtim
	stat.Birthtim = filetimeToTimespec(info.CreationTime)
	stat.Dev = uint64(info.VolumeSerialNumber)
	stat.Ino = uint64(info.FileIndexHigh)<<32 | uint64(info.FileIndexLow)
	stat.Nlink = info.NumberOfLinks
	if stat.Nlink == 0 {
		stat.Nlink = 1
	}
	stat.Uid = 0
	stat.Gid = 0
	stat.Flags = attributesToFlags(info.FileAttributes)
	applyReadonlyMode(stat, info.FileAttributes)
	setBlocks(stat)
}

// casePath 获取真实大小写路径
// 入参: path 路径
// 返回: string 真实路径, error 错误对象
func (s *SymFS) casePath(path string) (string, error) {
	path = strings.ReplaceAll(path, "\\", "/")
	path = strings.Trim(path, "/")
	if path == "" {
		return "/", nil
	}
	dir := s.root
	parts := strings.Split(path, "/")
	actual := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" || part == "." {
			continue
		}
		if part == ".." {
			return "", windows.ERROR_INVALID_NAME
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			return "", err
		}
		name := ""
		for _, entry := range entries {
			if strings.EqualFold(entry.Name(), part) {
				name = entry.Name()
				break
			}
		}
		if name == "" {
			return "", windows.ERROR_FILE_NOT_FOUND
		}
		actual = append(actual, name)
		dir = filepath.Join(dir, name)
	}
	return "/" + strings.Join(actual, "/"), nil
}

// fillStat 填充统计信息
// 入参: stat 统计信息结构体指针, fi 文件信息接口
func (s *SymFS) fillStat(stat *fuse.Stat_t, fi os.FileInfo) {
	*stat = fuse.Stat_t{}
	stat.Size = fi.Size()
	stat.Mtim = fuse.NewTimespec(fi.ModTime())
	if sys, ok := fi.Sys().(*syscall.Win32FileAttributeData); ok {
		stat.Atim = fuse.NewTimespec(time.Unix(0, sys.LastAccessTime.Nanoseconds()))
		stat.Birthtim = fuse.NewTimespec(time.Unix(0, sys.CreationTime.Nanoseconds()))
		stat.Ctim = stat.Mtim
		stat.Flags = attributesToFlags(sys.FileAttributes)
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
	if sys, ok := fi.Sys().(*syscall.Win32FileAttributeData); ok {
		applyReadonlyMode(stat, sys.FileAttributes)
	}
	setBlocks(stat)
}
