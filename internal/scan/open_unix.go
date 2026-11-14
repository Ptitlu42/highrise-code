//go:build unix

package scan

import "syscall"

const open_flags = syscall.O_NOFOLLOW | syscall.O_NONBLOCK | syscall.O_CLOEXEC
