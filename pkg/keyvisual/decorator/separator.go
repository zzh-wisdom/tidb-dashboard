package decorator

import (
	"strings"
	"sync/atomic"

	"github.com/pingcap/log"
	"go.uber.org/zap"

	"github.com/pingcap-incubator/tidb-dashboard/pkg/config"
)

// NaiveLabelStrategy is one of the simplest LabelStrategy.
type separatorLabelStrategy struct {
	Separator atomic.Value
}

func SeparatorLabelStrategy(cfg *config.Config) LabelStrategy {
	s := &separatorLabelStrategy{}
	s.Separator.Store(cfg.KVSeparator)
	return s
}

// ReloadConfig reset separator
func (s *separatorLabelStrategy) ReloadConfig(cfg *config.Config) {
	s.Separator.Store(cfg.KVSeparator)
	log.Debug("Reload config", zap.String("separator", cfg.KVSeparator))
}

// CrossBorder is temporarily not considering cross-border logic
func (s *separatorLabelStrategy) CrossBorder(startKey, endKey string) bool {
	return false
}

// Label uses separator to split key
func (s *separatorLabelStrategy) Label(key string) (label LabelKey) {
	label.Key = key
	separator := s.Separator.Load().(string)
	if separator == "" {
		label.Labels = []string{key}
		return
	}
	label.Labels = strings.Split(key, separator)
	return
}

func (s *separatorLabelStrategy) LabelGlobalStart() LabelKey {
	return s.Label("")
}

func (s *separatorLabelStrategy) LabelGlobalEnd() LabelKey {
	return s.Label("")
}
