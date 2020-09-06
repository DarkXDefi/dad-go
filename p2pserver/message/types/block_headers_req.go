/*
 * Copyright (C) 2018 The dad-go Authors
 * This file is part of The dad-go library.
 *
 * The dad-go is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Lesser General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * The dad-go is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Lesser General Public License for more details.
 *
 * You should have received a copy of the GNU Lesser General Public License
 * along with The dad-go.  If not, see <http://www.gnu.org/licenses/>.
 */

package types

import (
	"bytes"
	"encoding/binary"

	"github.com/ontio/dad-go/p2pserver/common"
)

type HeadersReq struct {
	Hdr MsgHdr
	P   struct {
		Len       uint8
		HashStart [common.HASH_LEN]byte
		HashEnd   [common.HASH_LEN]byte
	}
}

//Check whether header is correct
func (this HeadersReq) Verify(buf []byte) error {
	err := this.Hdr.Verify(buf)
	return err
}

//Serialize message payload
func (this HeadersReq) Serialization() ([]byte, error) {
	p := new(bytes.Buffer)
	err := binary.Write(p, binary.LittleEndian, &(this.P))
	if err != nil {
		return nil, err
	}

	s := CheckSum(p.Bytes())
	this.Hdr.Init("getheaders", s, uint32(len(p.Bytes())))

	hdrBuf, err := this.Hdr.Serialization()
	if err != nil {
		return nil, err
	}
	buf := bytes.NewBuffer(hdrBuf)
	err = binary.Write(buf, binary.LittleEndian, p.Bytes())
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), err
}

//Deserialize message payload
func (this *HeadersReq) Deserialization(p []byte) error {
	buf := bytes.NewBuffer(p)
	err := binary.Read(buf, binary.LittleEndian, &(this.Hdr))
	if err != nil {
		return err
	}

	err = binary.Read(buf, binary.LittleEndian, &(this.P.Len))
	if err != nil {
		return err
	}

	err = binary.Read(buf, binary.LittleEndian, &(this.P.HashStart))
	if err != nil {
		return err
	}

	err = binary.Read(buf, binary.LittleEndian, &(this.P.HashEnd))
	return err
}
