// SPDX-License-Identifier: GPL-3.0-only
package core

import "time"

var ClockPositions = [][2]string{{"topLeft", "左上角"}, {"topCenter", "顶部居中"}, {"topRight", "右上角"}, {"centerLeft", "左侧居中"}, {"center", "正中间"}, {"centerRight", "右侧居中"}, {"bottomLeft", "左下角"}, {"bottomCenter", "底部居中"}, {"bottomRight", "右下角"}}

func ClockGrid(position string) (int, int) {
	for i, p := range ClockPositions {
		if p[0] == position {
			return i % 3, i / 3
		}
	}
	return 2, 0
}
func LocalClockText(now time.Time) string { return platformClockText(now) }
