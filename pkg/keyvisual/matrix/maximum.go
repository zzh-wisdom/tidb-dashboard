package matrix

import "github.com/pingcap-incubator/tidb-dashboard/pkg/keyvisual/decorator"

// 阈值模式
type maximumHelper struct {
}

type maximumStrategy struct {
	AxisDivideMode
	decorator.LabelStrategy
}

func MaximumStrategy(label decorator.LabelStrategy, axisCompactStrategy AxisDivideMode) Strategy {
	return &maximumStrategy{
		AxisDivideMode: axisCompactStrategy,
		LabelStrategy:  label,
	}
}

func (*maximumStrategy) GenerateHelper(axes []Axis, compactKeys []string) interface{} {
	return maximumHelper{}
}

func (*maximumStrategy) Split(dst, src Axis, tag SplitTag, axesIndex int, helper interface{}) {
	CheckPartOf(dst.Keys, src.Keys)

	if len(dst.Keys) == len(src.Keys) {
		switch tag {
		case SplitTo:
			copy(dst.Values, src.Values)
		case SplitAdd:
			for i, v := range src.Values {
				dst.Values[i] = MaxUint64(dst.Values[i], v)
			}
		default:
			panic("unreachable")
		}
		return
	}

	start := 0
	for startKey := src.Keys[0]; !equal(dst.Keys[start], startKey); {
		start++
	}
	end := start + 1

	switch tag {
	case SplitTo:
		for i, key := range src.Keys[1:] {
			for !equal(dst.Keys[end], key) {
				end++
			}
			value := src.Values[i] / uint64(end-start)
			for ; start < end; start++ {
				dst.Values[start] = value
			}
			end++
		}
	case SplitAdd:
		for i, key := range src.Keys[1:] {
			for !equal(dst.Keys[end], key) {
				end++
			}
			for ; start < end; start++ {
				dst.Values[start] = MaxUint64(dst.Values[start], src.Values[i])
			}
			end++
		}
	default:
		panic("unreachable")
	}
}

func (m *maximumStrategy) GetAxisDivideMode() AxisDivideMode {
	return m.AxisDivideMode
}
