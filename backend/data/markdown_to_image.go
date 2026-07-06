package data

import "blades-stock/backend/logger"

func SetAppIcon(data []byte) {
	if len(data) > 0 {
		logger.SugaredLogger.Infof("SetAppIcon成功, 大小: %d", len(data))
	}
}
