package matrix

import "github.com/pingcap-incubator/tidb-dashboard/pkg/keyvisual/decorator"

// 阈值模式
type maximumHelper struct {
}

type maximumStrategy struct {
	ChunkStrategy
	decorator.LabelStrategy
}

func MaximumStrategy(label decorator.LabelStrategy,chunkStrategy ChunkStrategy) Strategy {
	return &maximumStrategy{
		ChunkStrategy: chunkStrategy,
		LabelStrategy: label,
	}
}

func (*maximumStrategy) GenerateHelper(chunks []chunk, compactKeys []string) interface{} {
	return maximumHelper{}
}

func (*maximumStrategy) Split(dst, src chunk, tag splitTag, axesIndex int, helper interface{}) {
	CheckPartOf(dst.Keys, src.Keys)

	if len(dst.Keys) == len(src.Keys) {
		switch tag {
		case splitTo:
			copy(dst.Values, src.Values)
		case splitAdd:
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
	case splitTo:
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
	case splitAdd:
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

func (m *maximumStrategy) GetChunkStrategy() ChunkStrategy {
	return m.ChunkStrategy
}


