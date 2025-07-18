//go:build freebsd

package fingerprint

import (
	"fmt"
	"net"
	"syscall"
	"unsafe"
)

type tcpInfo struct {
	State          uint8
	Options        uint8
	SndScale       uint8
	RcvScale       uint8
	__pad          [4]byte
	Rto            uint32
	Ato            uint32
	SndMss         uint32
	RcvMss         uint32
	Unacked        uint32
	Sacked         uint32
	Lost           uint32
	Retrans        uint32
	Fackets        uint32
	Last_data_sent uint32
	Last_ack_sent  uint32
	Last_data_recv uint32
	Last_ack_recv  uint32
	Pmtu           uint32
	Rcv_ssthresh   uint32
	RTT            uint32
	RTTvar         uint32
	Snd_ssthresh   uint32
	Snd_cwnd       uint32
	Advmss         uint32
	Reordering     uint32
	Rcv_rtt        uint32
	Rcv_space      uint32
	Total_retrans  uint32
	Snd_wnd        uint32
	// Truncated for brevity — add more fields if needed
}

// AssignTCPFingerprint extracts TCP fingerprint information from a connection
func AssignTCPFingerprint(conn net.Conn) (*JA4T, error) {
	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		return nil, fmt.Errorf("not a TCPConn")
	}

	rawConn, err := tcpConn.SyscallConn()
	if err != nil {
		return nil, fmt.Errorf("SyscallConn failed: %w", err)
	}

	var info tcpInfo
	var sysErr error

	err = rawConn.Control(func(fd uintptr) {
		size := uint32(unsafe.Sizeof(info))
		_, _, errno := syscall.Syscall6(
			syscall.SYS_GETSOCKOPT,
			fd,
			uintptr(syscall.IPPROTO_TCP),
			uintptr(syscall.TCP_INFO),
			uintptr(unsafe.Pointer(&info)),
			uintptr(unsafe.Pointer(&size)),
			0,
		)
		if errno != 0 {
			sysErr = errno
		}
	})
	if err != nil {
		return nil, fmt.Errorf("SyscallConn.Control: %w", err)
	}
	if sysErr != nil {
		return nil, fmt.Errorf("getsockopt TCP_INFO: %w", sysErr)
	}

	fp := &JA4T{
		Window:      info.Snd_wnd,
		MSS:         uint16(info.SndMss),
		WindowScale: info.SndScale,
	}

	const (
		TCPI_OPT_TIMESTAMPS = 1 << 0
		TCPI_OPT_SACK       = 1 << 1
		TCPI_OPT_WSCALE     = 1 << 2
	)

	if info.Options&TCPI_OPT_SACK != 0 {
		fp.Options = append(fp.Options, 4, 1)
	}
	if info.Options&TCPI_OPT_TIMESTAMPS != 0 {
		fp.Options = append(fp.Options, 8, 1)
	}
	if info.Options&TCPI_OPT_WSCALE != 0 {
		fp.Options = append(fp.Options, 3)
	}

	return fp, nil
}
