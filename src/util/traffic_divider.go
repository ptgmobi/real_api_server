package util

import (
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"strings"
)

var (
	ErrNotCompiled = errors.New("the divider is not compiled")
	ErrHasCompiled = errors.New("the divider has been compiled")
	ErrNoObj       = errors.New("No object adds to divider")
)

type Divider struct {
	sum        float64
	weightList []float64
	objList    []interface{}
	labelList  []string
	compiled   bool
}

func NewDivider() *Divider {
	return &Divider{
		sum:        0,
		weightList: make([]float64, 0, 8),
		objList:    make([]interface{}, 0, 8),
		labelList:  make([]string, 0, 8),
		compiled:   false,
	}
}

func (div *Divider) AddObj(weight float64, obj interface{}, label ...string) error {
	if div.compiled {
		return ErrHasCompiled
	}
	div.sum += weight

	var lastWeight float64 = 0.0
	length := len(div.weightList)
	if length > 0 {
		lastWeight = div.weightList[length-1]
	}
	div.weightList = append(div.weightList, lastWeight+weight)

	div.objList = append(div.objList, obj)

	var l string
	if len(label) > 0 {
		l = label[0]
	}
	div.labelList = append(div.labelList, l)
	return nil
}

func (div *Divider) Compile() error {
	if div.compiled {
		return ErrHasCompiled
	}
	if len(div.weightList) == 0 {
		return ErrNoObj
	}

	// normalize
	for i := 0; i != len(div.weightList); i++ {
		div.weightList[i] /= div.sum
	}
	div.compiled = true
	return nil
}

func (div *Divider) Dump() string {
	if !div.compiled {
		return ""
	}
	list := make([]string, len(div.weightList))
	for i := 0; i != len(div.weightList); i++ {
		list[i] = fmt.Sprintf("%s: %.6f", div.labelList[i], div.weightList[i])
	}
	return "|| " + strings.Join(list, " | ") + " ||"
}

func (div *Divider) GetObj() (obj interface{}, label string, err error) {
	if !div.compiled {
		err = ErrNotCompiled
		return
	}
	r := rand.Float64()
	i := sort.SearchFloat64s(div.weightList, r)
	return div.objList[i], div.labelList[i], nil
}
