package util

import (
	"fmt"
	"math"

	"github.com/jaypipes/ghw/pkg/unitutil"
)

func FormatBytes(b int64) string {
	unit, unitStr := unitutil.AmountString(b)
	bf := int64(math.Ceil(float64(b) / float64(unit)))
	return fmt.Sprintf("%d%s", bf, unitStr)
}
