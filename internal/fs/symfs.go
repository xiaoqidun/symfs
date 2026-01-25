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
	"path/filepath"

	"github.com/winfsp/cgofuse/fuse"
)

// SymFS 透传文件系统
type SymFS struct {
	fuse.FileSystemBase
	root string
	host *fuse.FileSystemHost
}

// NewSymFS 创建 SymFS 实例
// 入参: root 源目录路径
// 返回: SymFS 实例指针
func NewSymFS(root string) *SymFS {
	return &SymFS{root: root}
}

// SetHost 设置 FUSE 主机
// 入参: host FUSE 主机实例
func (s *SymFS) SetHost(host *fuse.FileSystemHost) {
	s.host = host
}

// realPath 获取真实路径
// 入参: path 相对路径
// 返回: string 绝对路径
func (s *SymFS) realPath(path string) string {
	return filepath.Join(s.root, path)
}
