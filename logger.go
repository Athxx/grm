package grm

import (
	"fmt"
	"log"
)

func init() {
	//Set the default log display information, display file and line number.
	log.SetFlags(log.Llongfile | log.LstdFlags)
}

var (
	//LogDepth Log Call Depth Record the log call level, used to locate the business layer code
	LogDepth                                         = 4
	LogErr   func(err string) error                  = logErr
	LogSQL   func(sqlStr string, args []interface{}) = logSQL
)

//logErr Record error log
func logErr(err string) error {
	return log.Output(LogDepth, err)
}

//logSQL Print sql statement and parameters
func logSQL(sqlStr string, args []interface{}) {
	if args != nil {
		log.Output(LogDepth, fmt.Sprintln("sql:", sqlStr, ",args:", args))
	} else {
		log.Output(LogDepth, "sql:"+sqlStr)
	}
}
