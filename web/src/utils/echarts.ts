// echarts 按需引入：全项目共享注册，压缩打包体积（全量 ~1.09MB -> 按需 ~450KB）
// 新增图表/组件类型时在此补充注册，勿在各页面直接全量 import 'echarts'
import { use } from 'echarts/core'
import { BarChart, GraphChart, LineChart, PieChart } from 'echarts/charts'
import {
  DataZoomComponent,
  GridComponent,
  LegendComponent,
  TitleComponent,
  TooltipComponent,
} from 'echarts/components'
import { CanvasRenderer, SVGRenderer } from 'echarts/renderers'

use([
  LineChart,
  BarChart,
  GraphChart,
  PieChart,
  GridComponent,
  TooltipComponent,
  LegendComponent,
  TitleComponent,
  DataZoomComponent,
  CanvasRenderer,
  SVGRenderer,
])

export * from 'echarts/core'
