// Copyright 2017 ETH Zurich
// Copyright 2018 ETH Zurich, Anapaya Systems
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmn

import (
	"fmt"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/ctrl/path_mgmt"
	"github.com/scionproto/scion/go/lib/log"
	"github.com/scionproto/scion/go/lib/scmp"
	_ "github.com/scionproto/scion/go/lib/scrypto" // Make sure math/rand is seeded
	"github.com/scionproto/scion/go/lib/serrors"
	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/lib/spkt"
	"github.com/scionproto/scion/go/lib/topology"
)

const (
	DefaultInterval = 1 * time.Second
	DefaultTimeout  = 2 * time.Second
	MaxEchoes       = 1 << 16
)

type ScmpStats struct {
	// sent is the number of sent packets
	Sent uint
	// recv is the number of received packets
	Recv uint
}

var (
	Count         uint16
	PayloadSize   uint
	Interactive   bool
	Interval      time.Duration
	Timeout       time.Duration
	LocalIPString string
	Remote        snet.UDPAddr
)

var (
	LocalIA   addr.IA
	LocalIP   net.IP
	Conn      net.PacketConn
	Mtu       uint16
	PathEntry snet.Path
	Stats     *ScmpStats
	Start     time.Time
)

func ValidateFlags() {
	if LocalIPString != "" {
		LocalIP = net.ParseIP(LocalIPString)
		if LocalIP == nil {
			Fatal("Invalid local address")
		}
	}
	// scmp-tool does not use ports, so we ignore them
	if Interval == 0 {
		Interval = 1
	}
}

func NewSCMPPkt(t scmp.Type, info scmp.Info, ext common.Extension) *spkt.ScnPkt {
	var exts []common.Extension
	scmpMeta := scmp.Meta{InfoLen: uint8(info.Len() / common.LineLen)}
	pld := make(common.RawBytes, scmp.MetaLen+uint(info.Len())+PayloadSize)
	scmpMeta.Write(pld)
	info.Write(pld[scmp.MetaLen:])
	scmpHdr := scmp.NewHdr(scmp.ClassType{Class: scmp.C_General, Type: t}, len(pld))
	if ext != nil {
		exts = []common.Extension{ext}
	}
	pkt := &spkt.ScnPkt{
		DstIA:   Remote.IA,
		SrcIA:   LocalIA,
		DstHost: addr.HostFromIP(Remote.Host.IP),
		SrcHost: addr.HostFromIP(LocalIP),
		Path:    Remote.Path,
		HBHExt:  exts,
		L4:      scmpHdr,
		Pld:     pld,
	}
	return pkt
}

func NextHopAddr() net.Addr {
	var nhAddr *net.UDPAddr
	if Remote.NextHop == nil {
		nhAddr = &net.UDPAddr{
			IP:   Remote.Host.IP,
			Port: topology.EndhostPort,
		}
	} else {
		nhAddr = Remote.NextHop
	}
	return nhAddr
}

func Validate(pkt *spkt.ScnPkt) (*scmp.Hdr, *scmp.Payload, error) {
	scmpHdr, ok := pkt.L4.(*scmp.Hdr)
	if !ok {
		return nil, nil,
			common.NewBasicError("Not an SCMP header", nil, "type", common.TypeOf(pkt.L4))
	}
	scmpPld, ok := pkt.Pld.(*scmp.Payload)
	if !ok {
		return scmpHdr, nil,
			common.NewBasicError("Not an SCMP payload", nil, "type", common.TypeOf(pkt.Pld))
	}
	if scmpHdr.Class != scmp.C_Path || scmpHdr.Type != scmp.T_P_RevokedIF {
		return scmpHdr, scmpPld, nil
	}
	// Handle revocation
	infoRev, ok := scmpPld.Info.(*scmp.InfoRevocation)
	if !ok {
		return scmpHdr, scmpPld,
			serrors.New("Failed to parse SCMP revocation Info")
	}
	signedRevInfo, err := path_mgmt.NewSignedRevInfoFromRaw(infoRev.RawSRev)
	if err != nil {
		return scmpHdr, scmpPld,
			serrors.New("Failed to decode SCMP signed revocation Info")
	}
	ri, err := signedRevInfo.RevInfo()
	if err != nil {
		return scmpHdr, scmpPld,
			serrors.New("Failed to decode SCMP revocation Info")
	}
	return scmpHdr, scmpPld, common.NewBasicError("", nil, "Revocation", ri)
}

func Rand() uint64 {
	return rand.Uint64()
}

func UpdatePktTS(pkt *spkt.ScnPkt, ts time.Time) {
	scmpHdr := pkt.L4.(*scmp.Hdr)
	scmpHdr.SetTime(ts)
}

func Fatal(msg string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, "CRIT: "+msg+"\n", a...)
	os.Exit(1)
}

func SetupSignals(f func()) {
	sig := make(chan os.Signal, 2)
	signal.Notify(sig, os.Interrupt)
	signal.Notify(sig, syscall.SIGTERM)
	go func() {
		defer log.HandlePanic()
		<-sig
		if f != nil {
			f()
		}
		os.Exit(0)
	}()
}
