package addrgen

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Abdullah1738/juno-addrgen/internal/ffi"
)

type ErrorCode string

const (
	ErrUFVKEmpty               ErrorCode = "ufvk_empty"
	ErrUFVKInvalidBech32m      ErrorCode = "ufvk_invalid_bech32m"
	ErrUFVKHrpMismatch         ErrorCode = "ufvk_hrp_mismatch"
	ErrUFVKTlvInvalid          ErrorCode = "ufvk_tlv_invalid"
	ErrUFVKTypecodeUnsupported ErrorCode = "ufvk_typecode_unsupported"
	ErrUFVKValueLenInvalid     ErrorCode = "ufvk_value_len_invalid"
	ErrUFVKFVKBytesInvalid     ErrorCode = "ufvk_fvk_bytes_invalid"
	ErrCountZero               ErrorCode = "count_zero"
	ErrCountTooLarge           ErrorCode = "count_too_large"
	ErrRangeOverflow           ErrorCode = "range_overflow"
	ErrInternal                ErrorCode = "internal"
)

type Error struct {
	Code ErrorCode
}

func (e *Error) Error() string {
	return fmt.Sprintf("addrgen: %s", e.Code)
}

func (e *Error) CodeString() string {
	return string(e.Code)
}

func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	if !ok {
		return false
	}
	return e.Code == t.Code
}

func Derive(ufvk string, index uint32) (string, error) {
	raw, err := ffi.DeriveJSON(ufvk, index)
	if err != nil {
		return "", err
	}

	var resp deriveResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return "", errors.New("addrgen: invalid response")
	}

	switch resp.Status {
	case "ok":
		if resp.Address == "" {
			return "", errors.New("addrgen: invalid response")
		}
		return resp.Address, nil
	case "err":
		if resp.Error == "" {
			return "", errors.New("addrgen: invalid response")
		}
		return "", &Error{Code: ErrorCode(resp.Error)}
	default:
		return "", errors.New("addrgen: invalid response")
	}
}

func Batch(ufvk string, start uint32, count uint32) ([]string, error) {
	raw, err := ffi.BatchJSON(ufvk, start, count)
	if err != nil {
		return nil, err
	}

	var resp batchResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return nil, errors.New("addrgen: invalid response")
	}

	switch resp.Status {
	case "ok":
		if len(resp.Addresses) == 0 {
			return nil, errors.New("addrgen: invalid response")
		}
		if resp.Start != start || resp.Count != count {
			return nil, errors.New("addrgen: invalid response")
		}
		return resp.Addresses, nil
	case "err":
		if resp.Error == "" {
			return nil, errors.New("addrgen: invalid response")
		}
		return nil, &Error{Code: ErrorCode(resp.Error)}
	default:
		return nil, errors.New("addrgen: invalid response")
	}
}

type deriveResponse struct {
	Status  string `json:"status"`
	Address string `json:"address,omitempty"`
	Error   string `json:"error,omitempty"`
}

type batchResponse struct {
	Status    string   `json:"status"`
	Start     uint32   `json:"start,omitempty"`
	Count     uint32   `json:"count,omitempty"`
	Addresses []string `json:"addresses,omitempty"`
	Error     string   `json:"error,omitempty"`
}
