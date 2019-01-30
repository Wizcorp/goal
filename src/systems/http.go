package systems

import (
	"context"
	"io"
	"net/http"
	"path"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"

	. "github.com/Wizcorp/goal/src/api"
)

type GoalHTTP interface {
	GoalSystem
	http.Handler
	HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request))
	Handle(pattern string, handler http.Handler)
}

type GoalMessageStreamConnection interface {
	NextWriter(messageType int) (io.WriteCloser, error)
	WriteMessage(messageType int, data []byte) error
}

type httpServer struct {
	Address  string
	Prefix   string
	Server   http.Server
	Services GoalServices
	Mux      *http.ServeMux
	Logger   GoalLogger
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func NewHTTP() *httpServer {
	return &httpServer{
		Mux: http.NewServeMux(),
	}
}

func (httpServer *httpServer) Setup(server GoalServer, config *GoalConfig) error {
	prefix := config.String("prefix", "/")
	addr := config.String("listen", "127.0.0.1:8080")
	messages := config.String("messages", "/messages")

	httpServer.Logger = (*server.GetSystem("logger")).(GoalLogger)
	logger := httpServer.Logger.GetInstance()
	logger.WithFields(LogFields{
		"address": addr,
		"prefix":  prefix,
	}).Info("Setting up HTTP Server system")

	httpServer.Services = (*server.GetSystem("services")).(GoalServices)
	for servicePath, service := range *httpServer.Services.GetServiceServers() {
		logger.WithFields(LogFields{
			"subpath": servicePath,
		}).Debug("Exposing service")

		httpServer.Handle(path.Join(prefix, servicePath)+"/", service)
	}

	httpServer.HandleFunc(path.Join(prefix, messages), httpServer.handleWebsocket)

	httpServer.Server = http.Server{
		Addr:    addr,
		Handler: httpServer,
	}

	go httpServer.Server.ListenAndServe()

	return nil
}

func (httpServer *httpServer) Teardown(server GoalServer, config *GoalConfig) error {
	timeout := config.Int64("shutdownTimeout", 10)
	ctx, cancel := context.WithTimeout(context.Background(), (time.Duration)(timeout)*time.Second)
	defer cancel()

	return httpServer.Server.Shutdown(ctx)
}

func (httpServer *httpServer) GetStatus() Status {
	return UpStatus
}

func (httpServer *httpServer) HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	httpServer.Mux.HandleFunc(pattern, handler)
}

func (httpServer *httpServer) Handle(pattern string, handler http.Handler) {
	httpServer.Mux.Handle(pattern, handler)
}

func (httpServer *httpServer) handleWebsocket(w http.ResponseWriter, r *http.Request) {
	contentTypes := r.Header["Content-Type"]
	conn, err := upgrader.Upgrade(w, r, nil)
	logger := httpServer.Logger.GetInstance()

	if err != nil {
		logger.WithFields(LogFields{
			"error": err,
		}).Warn("Error during upgrade to WebSocket protocol")

		return
	}

	var contentType string
	if len(contentTypes) == 0 {
		contentType = "application/json"
	} else {
		contentType = contentTypes[0]
	}

	switch contentType {
	case "application/json":
		go httpServer.processJSONMessages(conn)
	case "application/protobuf":
		go httpServer.processProtobufMessages(conn)
	default:
		logger.WithFields(LogFields{
			"remote":       conn.RemoteAddr().Network(),
			"content-type": contentType,
		}).Warn("Attempting to create message stream with invalid content type")
		conn.WriteMessage(0, []byte("Invalid Content-Type header"))
		conn.Close()
	}

}

func (httpServer *httpServer) processJSONMessages(conn *websocket.Conn) {
	logger := httpServer.Logger.GetInstance()
	ctx := createContext(conn, httpServer.Services.EmitJSONMessages)

	for {
		data, err := readConnectionData(conn, logger)

		if err != nil {
			// Todo: send error message before closing
			conn.Close()
			break
		}

		httpServer.Services.ProcessJSONMessages(ctx, *data)
	}
}

func (httpServer *httpServer) processProtobufMessages(conn *websocket.Conn) {
	logger := httpServer.Logger.GetInstance()
	ctx := createContext(conn, httpServer.Services.EmitProtobufMessages)

	for {
		data, err := readConnectionData(conn, logger)

		if err != nil {
			// Todo: send error message before closing
			conn.Close()
			break
		}

		if data == nil {
			break
		}

		httpServer.Services.ProcessProtobufMessages(ctx, *data)
	}
}

func (httpServer *httpServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	httpServer.Mux.ServeHTTP(w, r)
}

func readConnectionData(conn *websocket.Conn, logger *logrus.Logger) (*[]byte, error) {
	_, data, err := conn.ReadMessage()

	if err != nil {
		if ce, ok := err.(*websocket.CloseError); ok {
			switch ce.Code {
			case websocket.CloseNormalClosure,
				websocket.CloseGoingAway,
				websocket.CloseNoStatusReceived:
				return nil, nil
			}
		}

		logger.WithFields(LogFields{
			"remote": conn.RemoteAddr().Network(),
			"error":  err,
		}).Error("Unexpected message stream read error")

		return nil, err
	}

	return &data, nil
}

func createContext(conn *websocket.Conn, emitter GoalServiceEmitter) context.Context {
	ctx := context.Background()
	ctx = context.WithValue(ctx, "conn", conn)
	ctx = context.WithValue(ctx, "emitter", emitter)

	return ctx
}
