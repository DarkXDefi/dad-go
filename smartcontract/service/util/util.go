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
package util

import (
	"errors"
	"github.com/ontio/dad-go/common"
	"github.com/ontio/dad-go/core/utils"
	"github.com/ontio/dad-go/vm/crossvm_codec"
)

//create paramters for neovm contract
func CreateNeoInvokeParam(contractAddress common.Address, input []byte) ([]byte, error) {
	params, err := crossvm_codec.DeserializeCallParam(input)
	if err != nil {
		return nil, err
	}

	list, ok := params.([]interface{})
	if ok == false {
		return nil, errors.New("invoke neovm param is not list type")
	}

	return utils.BuildNeoVMInvokeCode(contractAddress, list)
}
