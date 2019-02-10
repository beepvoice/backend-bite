package main

import (
  "bytes"
  "encoding/binary"
  "errors"
  "regexp"
)

var ExtractKeyParseError = errors.New("ExtractKey: parse error, possibly because seprator was not found")

// Marshal keys
func validObj(obj string) bool {
	return obj == "bite" || obj == "user"
}

// TODO: ensure security of regexp
var validConversationRegexp = regexp.MustCompile(`^[a-zA-Z0-9-]+$`)

func validConversation(conversation string) bool {
	return validConversationRegexp.MatchString(conversation)
}

const conversationSeprator = '@'
const objSeprator = '+'

func MarshalKey(obj, conversation string, start uint64) ([]byte, error) {
	prefixBytes, err := MarshalKeyPrefix(obj, conversation)
	if err != nil {
		return nil, err
	}

	startBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(startBytes, start)

	return append(prefixBytes, startBytes...), nil
}

func MarshalKeyPrefix(obj, conversation string) ([]byte, error) {
	if !validObj(obj) || !validConversation(conversation) {
		return nil, errors.New("main: FormatKey: bad obj or conversation")
	}
	return []byte(obj + string(objSeprator) + conversation + string(conversationSeprator)), nil
}

func ExtractKey(b []byte) (string, string, uint64, error) {
	startStart := bytes.LastIndexByte(b, conversationSeprator) + 1
	if startStart < 0 {
		return "", "", 0, ExtractKeyParseError
	}
	startBytes := b[startStart:]

	convStart := bytes.LastIndexByte(b[:startStart-1], objSeprator) + 1
	if convStart < 0 {
		return "", "", 0, ExtractKeyParseError
	}
	convBytes := b[convStart : startStart-1]

	objStart := 0
	if objStart < 0 {
		return "", "", 0, ExtractKeyParseError
	}
	objBytes := b[objStart : convStart-1]

	obj := string(objBytes)
	conv := string(convBytes)
	start := binary.BigEndian.Uint64(startBytes)

	return obj, conv, start, nil
}
