package matrix

type StrategyConfig struct {
	Mode StrategyMode

	DistanceStrategyRatio float64
	DistanceStrategyLevel int
	distanceStrategyCount int
}

func NewStrategyConfig(mode StrategyMode, distanceStrategyRatio float64, distanceStrategyLevel, distanceStrategyCount int) *StrategyConfig {
	return &StrategyConfig {
		mode,
		distanceStrategyRatio,
		distanceStrategyLevel,
		distanceStrategyCount,
	}
}