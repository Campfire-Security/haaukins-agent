// Copyright (c) 2018-2019 Aalborg University
// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

package exercise

import (
	"errors"
	"fmt"
	"math/rand"
	"time"
	"unsafe"
)

// TODO add comments
var (
	ErrInvalidFlagFormat = errors.New("Invalid flag format")
)

const (
	letterBytes        = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ12345679"
	letterIdxBits      = 6                    // 6 bits to represent a letter index
	letterIdxMask      = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax       = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
	flagUniqueChars    = 10
	flagNumCharsFormat = 16
)

var (
	TagEmptyErr = errors.New("Tag cannot be empty")
)

type InvalidTagSyntaxErr struct {
	tag string
}

func (ite *InvalidTagSyntaxErr) Error() string {
	return fmt.Sprintf("Invalid syntax for tag \"%s\", allowed syntax: %s", ite.tag, tagRawRegexp)
}

type EmptyVarErr struct {
	Var  string
	Type string
}

func (eve *EmptyVarErr) Error() string {
	if eve.Type == "" {
		return fmt.Sprintf("%s cannot be empty", eve.Var)
	}

	return fmt.Sprintf("%s cannot be empty for %s", eve.Var, eve.Type)
}

func NewTag(s string) (string, error) {
	t := s
	if err := ValidateTag(t); err != nil {
		return "", err
	}

	return t, nil
}

func ValidateTag(t string) error {
	if t == "" {
		return TagEmptyErr
	}

	if !tagRegex.MatchString(t) {
		return &InvalidTagSyntaxErr{t}
	}

	return nil
}

var flagSrc = rand.NewSource(time.Now().UnixNano())

func randCharBytes(n int) string {
	b := make([]byte, n)
	for i, cache, remain := n-1, flagSrc.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = flagSrc.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i--
		}
		cache >>= letterIdxBits
		remain--
	}

	return *(*string)(unsafe.Pointer(&b))
}

type Flag [flagNumCharsFormat]byte

func NewFlag() Flag {
	var arr [flagUniqueChars]byte
	s := []byte(randCharBytes(10))
	copy(arr[:], s)
	formattedFlag := formatFlag(arr)
	return Flag(formattedFlag)
}

func (f Flag) String() string {
	// Used only in dynamic flags
	var str string
	str = string(f[5 : flagNumCharsFormat-1])
	i := (2 + rand.Intn(2))
	j := (i + 2 + rand.Intn(2))

	return fmt.Sprintf("FIRE{%s}", str[:i]+"-"+str[i:j]+"-"+str[j:])
}

func formatFlag(arr [flagUniqueChars]byte) [flagNumCharsFormat]byte {
	flag := fmt.Sprintf("FIRE{%s}", arr)
	var formattedFlag [flagNumCharsFormat]byte
	for k, v := range []byte(flag) {
		formattedFlag[k] = byte(v)
	}
	return formattedFlag
}
