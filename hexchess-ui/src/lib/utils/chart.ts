import { Chart, Legend, LinearScale, LineController, LineElement, PointElement, TimeScale, Tooltip } from 'chart.js';

Chart.register(
	LineController,
	LineElement,
	PointElement,
	LinearScale,
	TimeScale,
	Tooltip,
	Legend
);