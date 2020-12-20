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

package global_params

import (
	"io"

	"fmt"
	"github.com/ontio/dad-go/common"
	"github.com/ontio/dad-go/common/serialization"
	"github.com/ontio/dad-go/errors"
)

type Param struct {
	Key   string
	Value string
}

type Params []*Param

type Admin common.Address

type ParamNameList []string

func (params *Params) SetParam(value *Param) {
	for index, param := range *params {
		if param.Key == value.Key {
			(*params)[index] = value
			return
		}
	}
	*params = append(*params, value)
}

func (params *Params) GetParam(key string) (int, *Param) {
	for index, param := range *params {
		if param.Key == key {
			return index, param
		}
	}
	return -1, nil
}

func (params *Params) Serialize(w io.Writer) error {
	paramNum := len(*params)
	if err := serialization.WriteVarUint(w, uint64(paramNum)); err != nil {
		return errors.NewDetailErr(err, errors.ErrNoCode, "param config, serialize params length error!")
	}
	for _, param := range *params {
		if err := serialization.WriteString(w, param.Key); err != nil {
			return errors.NewDetailErr(err, errors.ErrNoCode, fmt.Sprintf("param config, serialize param key %v error!", param.Key))
		}
		if err := serialization.WriteString(w, param.Value); err != nil {
			return errors.NewDetailErr(err, errors.ErrNoCode, fmt.Sprintf("param config, serialize param value %v error!", param.Value))
		}
	}
	return nil
}

func (params *Params) Deserialize(r io.Reader) error {
	paramNum, err := serialization.ReadVarUint(r, 0)
	if err != nil {
		return errors.NewDetailErr(err, errors.ErrNoCode, "param config, deserialize params length error!")
	}
	for i := 0; uint64(i) < paramNum; i++ {
		param := new(Param)
		param.Key, err = serialization.ReadString(r)
		if err != nil {
			return errors.NewDetailErr(err, errors.ErrNoCode, fmt.Sprintf("param config, deserialize param key %v error!", param.Key))
		}
		param.Value, err = serialization.ReadString(r)
		if err != nil {
			return errors.NewDetailErr(err, errors.ErrNoCode, fmt.Sprintf("param config, deserialize param value %v error!", param.Value))
		}
		*params = append(*params, param)
	}
	return nil
}

func (admin *Admin) Serialize(w io.Writer) error {
	_, err := w.Write(admin[:])
	if err != nil {
		return errors.NewDetailErr(err, errors.ErrNoCode, "param config, serialize admin error!")
	}
	return nil
}

func (admin *Admin) Deserialize(r io.Reader) error {
	_, err := io.ReadFull(r, admin[:])
	if err != nil {
		return errors.NewDetailErr(err, errors.ErrNoCode, "param config, deserialize admin error!")
	}
	return nil
}

func (nameList *ParamNameList) Serialize(w io.Writer) error {
	nameNum := len(*nameList)
	if err := serialization.WriteVarUint(w, uint64(nameNum)); err != nil {
		return errors.NewDetailErr(err, errors.ErrNoCode, "param config, serialize param name list length error!")
	}
	for _, value := range *nameList {
		if err := serialization.WriteString(w, value); err != nil {
			return errors.NewDetailErr(err, errors.ErrNoCode, fmt.Sprintf("param config, serialize param name %v error!", value))
		}
	}
	return nil
}

func (nameList *ParamNameList) Deserialize(r io.Reader) error {
	nameNum, err := serialization.ReadVarUint(r, 0)
	if err != nil {
		return errors.NewDetailErr(err, errors.ErrNoCode, "param config, deserialize param name list length error!")
	}
	for i := 0; uint64(i) < nameNum; i++ {
		name, err := serialization.ReadString(r)
		if err != nil {
			return errors.NewDetailErr(err, errors.ErrNoCode, fmt.Sprintf("param config, deserialize param name %v error!", name))
		}
		*nameList = append(*nameList, name)
	}
	return nil
}
