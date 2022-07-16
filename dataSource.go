package grm

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// dataSource对象,隔离sql原生对象
// dataSource  Isolate sql native objects
type dataSource struct {
	*sql.DB
	//config *DBConfig
}

// DBConfig 数据库连接池的配置
// DateSourceConfig Database connection pool configuration
type DBConfig struct {
	//DSN DataSourceName Database connection string
	DSN string //
	//Database diver name:mysql,postgres,oracle,sqlserver,sqlite3,clickhouse corresponds to DBType,A database may have multiple drivers
	DriverName string
	//Database Type:mysql,postgresql,oracle,mssql,sqlite,clickhouse corresponds to DriverName,A database may have multiple drivers
	DBType string
	//ShowSQL Whether to print SQL, use grm.ShowSQL record sql
	ShowSQL bool
	//MaxOpenConns Maximum number of database connections, Default 50
	MaxOpenConns int
	//MaxIdleConns The maximum number of free connections to the database default 50
	MaxIdleConns int
	//MaxLifetime: (Connection survival time in seconds)Destroy and rebuild the connection after the default 600 seconds (10 minutes)
	//Prevent the database from actively disconnecting and causing dead connections. MySQL Default wait_timeout 28800 seconds
	MaxLifetime int

	Tag string // customize tag

	//事务隔离级别的默认配置,默认为nil
	DefaultTxOptions *sql.TxOptions

	//全局禁用事务,默认false.为了处理某些数据库不支持事务,比如clickhouse
	//禁用事务应该有驱动实现,不应该由orm实现
	//DisableTransaction bool

	//MockSQLDB 用于mock测试的入口,如果MockSQLDB不为nil,则不使用DSN,直接使用MockSQLDB
	//db, mock, err := sqlmock.New()
	//MockSQLDB *sql.DB

	//SeataGlobalTx seata-golang分布式的适配函数,返回ISeataGlobalTx接口的实现
	SeataGlobalTx func(ctx context.Context) (ISeataGlobalTx, context.Context, error)

	//使用现有的数据库连接,优先级高于DSN
	SQLDB *sql.DB
}

// newDataSource 创建一个新的datasource,内部调用,避免外部直接使用datasource
// newDAtaSource Create a new datasource and call it internally to avoid direct external use of the datasource
func newDataSource(config *DBConfig) (*dataSource, error) {

	if config == nil {
		return nil, errors.New("config cannot be nil")
	}

	if config.DriverName == "" {
		return nil, errors.New("DriverName cannot be empty")
	}
	if config.DBType == "" {
		return nil, errors.New("DBType cannot be empty")
	}
	var db *sql.DB
	var errSQLOpen error

	if config.SQLDB == nil { //没有已经存在的数据库连接,使用DSN初始化
		if config.DSN == "" {
			return nil, errors.New("DSN cannot be empty")
		}
		db, errSQLOpen = sql.Open(config.DriverName, config.DSN)
		if errSQLOpen != nil {
			return nil, LogErr("newDataSource-->open数据库打开失败: " + errSQLOpen.Error())
		}
	} else { //使用已经存在的数据库连接
		db = config.SQLDB
	}

	if config.MaxOpenConns == 0 {
		config.MaxOpenConns = 50
	}
	if config.MaxIdleConns == 0 {
		config.MaxIdleConns = 50
	}

	if config.MaxLifetime == 0 {
		config.MaxLifetime = 600
	}

	//设置数据库最大连接数
	//Set the maximum number of database connections
	db.SetMaxOpenConns(config.MaxOpenConns)
	//设置数据库最大空闲连接数
	//Set the maximum number of free connections to the database
	db.SetMaxIdleConns(config.MaxIdleConns)
	//连接存活秒时间. 默认600(10分钟)后连接被销毁重建.避免数据库主动断开连接,造成死连接.MySQL默认wait_timeout 28800秒(8小时)
	//(Connection survival time in seconds) Destroy and rebuild the connection after the default 600 seconds (10 minutes)
	//Prevent the database from actively disconnecting and causing dead connections. MySQL Default wait_timeout 28800 seconds
	db.SetConnMaxLifetime(time.Second * time.Duration(config.MaxLifetime))

	//验证连接
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, LogErr("ping database error: " + err.Error())
	}

	return &dataSource{db}, nil
}

//Transaction reference: https://www.jianshu.com/p/2a144332c3db

// const beginStatus = 1

// dbConnection Database session, native query or transaction.
type dbConnection struct {

	// 原生db
	// native db
	db *sql.DB
	// 原生事务
	// native Transaction
	tx *sql.Tx
	// 数据库配置
	cfg *DBConfig

	//commitSign   int8    // 提交标记,控制是否提交事务
	//rollbackSign bool    // 回滚标记,控制是否回滚事务
}

// beginTx Open transaction
func (dbConn *dbConnection) beginTx(ctx context.Context) error {
	//s.rollbackSign = true
	if dbConn.tx == nil {

		//设置事务配置,主要是隔离级别
		var txOptions *sql.TxOptions
		ctxTxOptions := ctx.Value(ctxTxOptKey)
		if ctxTxOptions != nil {
			txOptions, _ = ctxTxOptions.(*sql.TxOptions)
		} else {
			txOptions = dbConn.cfg.DefaultTxOptions
		}

		tx, err := dbConn.db.BeginTx(ctx, txOptions)
		if err != nil {
			err = errors.New("beginTx事务开启失败: " + err.Error())
			return err
		}
		dbConn.tx = tx
		//s.commitSign = beginStatus
		return nil
	}
	//s.commitSign++
	return nil
}

// rollback a transaction
func (dbConn *dbConnection) rollback() error {
	//if s.tx != nil && s.rollbackSign == true {
	if dbConn.tx != nil {
		err := dbConn.tx.Rollback()
		if err != nil {
			err = errors.New("rollback事务回滚失败: " + err.Error())
			return err
		}
		dbConn.tx = nil
		return nil
	}
	return nil
}

// commit transaction
func (dbConn *dbConnection) commit() error {
	//s.rollbackSign = false
	if dbConn.tx == nil {
		return errors.New("commit事务为空")
	}
	err := dbConn.tx.Commit()
	if err != nil {
		err = errors.New("commit事务提交失败: " + err.Error())
		return err
	}
	dbConn.tx = nil
	return nil
}

// execContext 执行sql语句,如果已经开启事务,就以事务方式执行,如果没有开启事务,就以非事务方式执行
// execContext Execute sql statement,If the transaction has been opened,it will be executed in transaction mode, if the transaction is not opened,it will be executed in non-transactional mode
func (dbConn *dbConnection) execCtx(ctx context.Context, execSql *string, args []interface{}) (*sql.Result, error) {

	//打印SQL
	//print SQL
	if dbConn.cfg.ShowSQL {
		//logger.Info("printSQL", logger.String("sql", execSql), logger.Any("args", args))
		LogSQL(*execSql, args)
	}

	if dbConn.tx != nil {
		res, resErr := dbConn.tx.ExecContext(ctx, *execSql, args...)
		return &res, resErr
	}
	res, resErr := dbConn.db.ExecContext(ctx, *execSql, args...)
	return &res, resErr
}

// queryRowCtx 如果已经开启事务,就以事务方式执行,如果没有开启事务,就以非事务方式执行
func (dbConn *dbConnection) queryRowCtx(ctx context.Context, query *string, args []interface{}) *sql.Row {
	//打印SQL
	if dbConn.cfg.ShowSQL {
		//logger.Info("printSQL", logger.String("sql", query), logger.Any("args", args))
		LogSQL(*query, args)
	}

	if dbConn.tx != nil {
		return dbConn.tx.QueryRowContext(ctx, *query, args...)
	}
	return dbConn.db.QueryRowContext(ctx, *query, args...)
}

// queryCtx 查询数据,如果已经开启事务,就以事务方式执行,如果没有开启事务,就以非事务方式执行
// queryRowCtx Execute sql  row statement,If the transaction has been opened,it will be executed in transaction mode, if the transaction is not opened,it will be executed in non-transactional mode
func (dbConn *dbConnection) queryCtx(ctx context.Context, query *string, args []interface{}) (*sql.Rows, error) {
	//打印SQL
	if dbConn.cfg.ShowSQL {
		//logger.Info("printSQL", logger.String("sql", query), logger.Any("args", args))
		LogSQL(*query, args)
	}

	if dbConn.tx != nil {
		return dbConn.tx.QueryContext(ctx, *query, args...)
	}
	return dbConn.db.QueryContext(ctx, *query, args...)
}

/*
// prepareContext 预执行,如果已经开启事务,就以事务方式执行,如果没有开启事务,就以非事务方式执行
// prepareContext Pre-execution,If the transaction has been opened,it will be executed in transaction mode,if the transaction is not opened,it will be executed in non-transactional mode
func (dbConn *dbConnection) prepareContext(ctx context.Context, query *string) (*sql.Stmt, error) {
	//打印SQL
	//print SQL
	if dbConn.cfg.ShowSQL {
		//logger.Info("printSQL", logger.String("sql", query))
		LogSQL(*query, nil)
	}

	if dbConn.tx != nil {
		return dbConn.tx.PrepareContext(ctx, *query)
	}
	return dbConn.db.PrepareContext(ctx, *query)
}
*/
