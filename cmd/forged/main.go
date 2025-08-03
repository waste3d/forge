package main

import (
	"fmt"
	"log"
	"net"
	"time"

	pb "github.com/waste3d/forge/proto"
	"google.golang.org/grpc"
)

type forgeServer struct {
	pb.UnimplementedForgeServer
}

func (s *forgeServer) Up(req *pb.UpRequest, stream pb.Forge_UpServer) error {
	log.Printf("Получен Up-запрос для приложения: %s", req.GetAppName())
	log.Printf("Содержимое конфига: \n%s", req.GetConfigContent())

	for i := 0; i < 5; i++ {
		entry := &pb.LogEntry{
			ServiceName: "forged-daemon",
			Timestamp:   time.Now().Unix(),
			Message:     fmt.Sprintf("...отправка тестового лога #%d", i+1),
		}
		if err := stream.Send(entry); err != nil {
			log.Printf("Ошибка при отправке лога в стрим: %v", err)
			return err
		}
		time.Sleep(500 * time.Millisecond)
	}
	log.Printf("Отправка логов для '%s' завершена.", req.GetAppName())
	return nil
}

// TODO: Реализовать метод down

func main() {
	lis, err := net.Listen("tcp", ":9001")
	if err != nil {
		log.Fatalf("Не удалось запустить listener: %v", err)
	}

	s := grpc.NewServer()
	pb.RegisterForgeServer(s, &forgeServer{})

	log.Println("Демон 'forged' запущен на порту :9001...")
	if err := s.Serve(lis); err != nil {
		log.Fatalf("Ошибка gRPC сервера: %v", err)
	}
}
