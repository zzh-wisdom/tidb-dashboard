package matrix

type StrategyConfig struct {
	HeatmapStrategy

	DistanceStrategyRatio float64
	DistanceStrategyLevel int
	distanceStrategyCount int
}

func NewStrategyConfig(heatmapStrategy HeatmapStrategy, distanceStrategyRatio float64, distanceStrategyLevel, distanceStrategyCount int) *StrategyConfig {
	return &StrategyConfig{
		heatmapStrategy,
		distanceStrategyRatio,
		distanceStrategyLevel,
		distanceStrategyCount,
	}
}