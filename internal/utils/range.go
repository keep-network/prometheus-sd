package utils

import (
	"fmt"
	"strconv"
	"strings"
)

type Range struct {
	Start, End int
}

func NewRange(rangeStr string) (result Range, err error) {
	splitResult := strings.Split(rangeStr, "-")
	if len(splitResult) != 2 {
		return result, fmt.Errorf("invalid range provided: %s", rangeStr)
	}

	result.Start, err = strconv.Atoi(splitResult[0])
	if err != nil {
		return result, fmt.Errorf("failed to convert string to int: %s", splitResult[0])
	}
	result.End, err = strconv.Atoi(splitResult[1])
	if err != nil {
		return result, fmt.Errorf("failed to convert string to int: %s", splitResult[1])
	}
	return
}
