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

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/winfsp/cgofuse/fuse"
	"github.com/xiaoqidun/symfs/internal/fs"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "用法: %s <源目录> <挂载点> [FUSE选项...]\n", os.Args[0])
		os.Exit(1)
	}
	source := filepath.Clean(os.Args[1])
	target := filepath.Clean(os.Args[2])
	source, err := filepath.Abs(source)
	if err != nil {
		fmt.Fprintf(os.Stderr, "无效的源目录: %v\n", err)
		os.Exit(1)
	}
	info, err := os.Stat(source)
	if err != nil || !info.IsDir() {
		fmt.Fprintf(os.Stderr, "无效的源目录: %s\n", source)
		os.Exit(1)
	}
	opts := append([]string{"-o", "uid=-1", "-o", "gid=-1"}, os.Args[3:]...)
	symfs := fs.NewSymFS(source)
	host := fuse.NewFileSystemHost(symfs)
	symfs.SetHost(host)
	host.SetCapReaddirPlus(true)
	if !host.Mount(target, opts) {
		fmt.Fprintf(os.Stderr, "无法完成挂载\n")
		os.Exit(1)
	}
}
