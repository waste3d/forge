package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/moby/term"
	"github.com/spf13/cobra"
	pb "github.com/waste3d/forge/internal/gen/proto"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

var (
	interactive bool
	tty         bool
)

var execCmd = &cobra.Command{
	Use: "exec <appName> <serviceName> -- [command...]",
	Long: `Выполняет команду в запущенном контейнере указанного сервиса.
	Для интерактивных сессий (например, 'sh' или 'bash') используйте флаги -i и -t.
	Пример: forge exec my-app backend -it -- /bin/bash`,
	Args: cobra.MinimumNArgs(3),
	Run:  runExec,
}

func init() {
	execCmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Держать STDIN открытым (для интерактивных сессий)")
	execCmd.Flags().BoolVarP(&tty, "tty", "t", false, "Выделить псевдо-терминал (для интерактивных сессий)")

	rootCmd.AddCommand(execCmd)
}

func runExec(cmd *cobra.Command, args []string) {
	appName := args[0]
	serviceName := args[1]
	command := args[2:]

	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()

	if err := runExecLogic(ctx, cancel, appName, serviceName, command); err != nil {
		if s, ok := status.FromError(err); ok && s.Code() == codes.Canceled {
			return
		}
		errorLog(os.Stderr, "ошибка при выполнении exec: %v\n", err)
		os.Exit(1)
	}
}

func runExecLogic(ctx context.Context, cancel context.CancelFunc, appName, serviceName string, command []string) error {
	conn, err := grpc.Dial(daemonAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("не удалось подключиться к серверу: %w", err)
	}
	defer conn.Close()

	client := pb.NewForgeClient(conn)
	stream, err := client.Exec(ctx)
	if err != nil {
		return fmt.Errorf("не удалось создать клиент: %w", err)
	}

	setup := &pb.ExecPayload{
		Payload: &pb.ExecPayload_Setup{
			Setup: &pb.ExecSetup{
				AppName:     appName,
				ServiceName: serviceName,
				Command:     command,
				Tty:         tty,
			},
		},
	}

	if err := stream.Send(setup); err != nil {
		return fmt.Errorf("не удалось отправить параметры exec: %w", err)
	}

	stdinFd := int(os.Stdin.Fd())
	var oldState *term.State
	if tty && term.IsTerminal(uintptr(stdinFd)) {
		oldState, err = term.MakeRaw(uintptr(stdinFd))
		if err != nil {
			return fmt.Errorf("не удалось перевести терминал в raw-режим: %w", err)
		}

		defer term.RestoreTerminal(uintptr(stdinFd), oldState)
	}

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		for {
			resp, err := stream.Recv()
			if err != nil {
				if err == io.EOF {
					return nil
				}
				return err
			}
			if _, err := os.Stdout.Write(resp.Data); err != nil {
				return err
			}

		}
	})

	if interactive {
		g.Go(func() error {
			buf := make([]byte, 4096)
			for {
				select {
				case <-gCtx.Done():
					return gCtx.Err()
				default:
					n, err := os.Stdin.Read(buf)
					if err != nil {
						if err == io.EOF {
							return stream.CloseSend()
						}
						return err
					}
					payload := &pb.ExecPayload{
						Payload: &pb.ExecPayload_Stdin{Stdin: buf[:n]},
					}
					if err := stream.Send(payload); err != nil {
						return err
					}
				}
			}
		})
	}

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		select {
		case <-gCtx.Done():
		case <-sigChan:
			cancel()
		}
		signal.Stop(sigChan)
	}()

	return g.Wait()
}
