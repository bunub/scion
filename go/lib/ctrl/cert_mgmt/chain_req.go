// Copyright 2017 ETH Zurich
// Copyright 2020 ETH Zurich, Anapaya Systems
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

// This file contains the Go representation of Certificate Chain requests.

package cert_mgmt

import (
	"fmt"
	"time"

	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/proto"
)

var _ proto.Cerealizable = (*ChainReq)(nil)

type ChainReq struct {
	RawIA        addr.IAInt `capnp:"isdas"`
	SubjectKeyID []byte
	RawDate      int64 `capnp:"date"`
}

func (c *ChainReq) IA() addr.IA {
	return c.RawIA.IA()
}

func (c *ChainReq) Date() time.Time {
	return time.Unix(c.RawDate, 0)
}

func (c *ChainReq) ProtoId() proto.ProtoIdType {
	return proto.CertChainReq_TypeID
}

func (c *ChainReq) String() string {
	return fmt.Sprintf("ISD-AS: %s SubjectKeyID: %x Date: %v", c.IA(), c.SubjectKeyID, c.Date())
}
