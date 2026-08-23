package app

import (
	"database/sql"

	"github.com/dzaneyo/relay/internal/repository"
	"github.com/dzaneyo/relay/internal/service"
)

type App struct {
	DB            *sql.DB
	Repo          *repository.Repository
	RecordService *service.RecordService
	RouteService  *service.RouteService
	Connect       *service.ConnectService
}

func New(db *sql.DB) *App {
	r := repository.New(db)
	return &App{
		DB:            db,
		Repo:          r,
		RecordService: service.NewRecordService(r),
		RouteService:  service.NewRouteService(r),
		Connect:       service.NewConnectService(r),
	}
}
