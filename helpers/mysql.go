package helpers

import (
	"react-base-service/conf"

	"react-base-service/golib/base"
	"gorm.io/gorm"
)

var MysqlClientLLM *gorm.DB

func InitMysql() {
	var err error
	for name, dbConf := range conf.RConf.Mysql {
		switch name {
		case "llm":
			MysqlClientLLM, err = base.InitMysqlClient(dbConf)
		}

		if err != nil {
			panic("mysql connect error: %v" + err.Error())
		}
	}
}
