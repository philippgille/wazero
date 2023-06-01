package sysfs

import (
	"net"
	"syscall"

	"github.com/tetratelabs/wazero/internal/fsapi"
	netapi "github.com/tetratelabs/wazero/internal/net"
	"github.com/tetratelabs/wazero/internal/platform"
)

func NewTCPListenerFile(tl *net.TCPListener) netapi.Sock {
	return &tcpListenerFile{tl: tl}
}

var _ netapi.Sock = (*tcpListenerFile)(nil)

type tcpListenerFile struct {
	fsapi.UnimplementedFile

	tl *net.TCPListener
}

// Accept implements the same method as documented on netapi.Sock
func (f *tcpListenerFile) Accept() (netapi.Conn, syscall.Errno) {
	conn, err := f.tl.Accept()
	if err != nil {
		return nil, platform.UnwrapOSError(err)
	}
	return &tcpConnFile{tc: conn.(*net.TCPConn)}, 0
}

// IsDir implements the same method as documented on File.IsDir
func (*tcpListenerFile) IsDir() (bool, syscall.Errno) {
	// We need to override this method because WASI-libc prestats the FD
	// and the default impl returns ENOSYS otherwise.
	return false, 0
}

// Close implements the same method as documented on fsapi.File
func (f *tcpListenerFile) Close() syscall.Errno {
	return platform.UnwrapOSError(f.tl.Close())
}

// Addr is exposed for testing.
func (f *tcpListenerFile) Addr() *net.TCPAddr {
	return f.tl.Addr().(*net.TCPAddr)
}

var _ netapi.Conn = (*tcpConnFile)(nil)

type tcpConnFile struct {
	fsapi.UnimplementedFile

	tc *net.TCPConn

	// closed is true when closed was called. This ensures proper syscall.EBADF
	closed bool
}

// IsDir implements the same method as documented on File.IsDir
func (*tcpConnFile) IsDir() (bool, syscall.Errno) {
	// We need to override this method because WASI-libc prestats the FD
	// and the default impl returns ENOSYS otherwise.
	return false, 0
}

// SetNonblock implements the same method as documented on fsapi.File
func (f *tcpConnFile) SetNonblock(enabled bool) (errno syscall.Errno) {
	syscallConn, err := f.tc.SyscallConn()
	if err != nil {
		return platform.UnwrapOSError(err)
	}

	// Prioritize the error from setNonblock over Control
	if controlErr := syscallConn.Control(func(fd uintptr) {
		errno = platform.UnwrapOSError(setNonblock(fd, enabled))
	}); errno == 0 {
		errno = platform.UnwrapOSError(controlErr)
	}
	return
}

// Read implements the same method as documented on fsapi.File
func (f *tcpConnFile) Read(buf []byte) (n int, errno syscall.Errno) {
	if n, errno = read(f.tc, buf); errno != 0 {
		// Defer validation overhead until we've already had an error.
		errno = fileError(f, f.closed, errno)
	}
	return
}

// Write implements the same method as documented on fsapi.File
func (f *tcpConnFile) Write(buf []byte) (n int, errno syscall.Errno) {
	if n, errno = write(f.tc, buf); errno != 0 {
		// Defer validation overhead until we've alwritey had an error.
		errno = fileError(f, f.closed, errno)
	}
	return
}

// Recvfrom implements the same method as documented on netapi.Conn
func (f *tcpConnFile) Recvfrom(p []byte, flags int) (n int, errno syscall.Errno) {
	if flags != MSG_PEEK {
		errno = syscall.EINVAL
		return
	}
	return recvfromPeek(f.tc, p)
}

// Shutdown implements the same method as documented on fsapi.Conn
func (f *tcpConnFile) Shutdown(how int) syscall.Errno {
	// FIXME: can userland shutdown listeners?
	var err error
	switch how {
	case syscall.SHUT_RD:
		err = f.tc.CloseRead()
	case syscall.SHUT_WR:
		err = f.tc.CloseWrite()
	case syscall.SHUT_RDWR:
		return f.close()
	default:
		return syscall.EINVAL
	}
	return platform.UnwrapOSError(err)
}

// Close implements the same method as documented on fsapi.File
func (f *tcpConnFile) Close() syscall.Errno {
	return f.close()
}

func (f *tcpConnFile) close() syscall.Errno {
	if f.closed {
		return 0
	}
	f.closed = true
	return f.Shutdown(syscall.SHUT_RDWR)
}
