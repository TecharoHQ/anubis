//go:build linux

package fingerprint

import (
	"fmt"
	"net"
	"syscall"
	"unsafe"
)

type tcpInfo struct {
	State                     uint8
	Ca_state                  uint8
	Retransmits               uint8
	Probes                    uint8
	Backoff                   uint8
	Options                   uint8
	Wnd_scale                 uint8
	Delivery_rate_app_limited uint8

	Rto    uint32
	Ato    uint32
	SndMss uint32
	RcvMss uint32

	Unacked uint32
	Sacked  uint32
	Lost    uint32
	Retrans uint32
	Fackets uint32

	Last_data_sent  uint32
	Last_ack_sent   uint32
	Last_data_recv  uint32
	Last_ack_recv   uint32
	PMTU            uint32
	Rcv_ssthresh    uint32
	RTT             uint32
	RTTvar          uint32
	Snd_ssthresh    uint32
	Snd_cwnd        uint32
	Advmss          uint32
	Reordering      uint32
	Rcv_rtt         uint32
	Rcv_space       uint32
	Total_retrans   uint32
	Pacing_rate     uint64
	Max_pacing_rate uint64
	Bytes_acked     uint64
	Bytes_received  uint64
	Segs_out        uint32
	Segs_in         uint32
	Notsent_bytes   uint32
	Min_rtt         uint32
	Data_segs_in    uint32
	Data_segs_out   uint32
	Delivery_rate   uint64
	Busy_time       uint64
	Rwnd_limited    uint64
	Sndbuf_limited  uint64
	Delivered       uint32
	Delivered_ce    uint32
	Bytes_sent      uint64
	Bytes_retrans   uint64
	DSACK_dups      uint32
	Reord_seen      uint32
	Rcv_ooopack     uint32
	Snd_wnd         uint32
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
		Window: info.Snd_wnd,
		MSS:    uint16(info.SndMss),
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
		fp.WindowScale = info.Wnd_scale
	}

	return fp, nil
}
