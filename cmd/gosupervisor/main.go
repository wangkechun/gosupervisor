package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/olekukonko/tablewriter"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/wangkechun/gosupervisor/pkg/process"
	pb "github.com/wangkechun/gosupervisor/pkg/proto"
	"google.golang.org/grpc"
)

var serverAddr string
var verbose bool

func main() {
	log.SetFlags(log.Lshortfile | log.LstdFlags)
	var cmdDaemon = &cobra.Command{
		Use:     "daemon CONFIG_PATH",
		Example: "daemon config.conf",
		Args:    cobra.MinimumNArgs(1),
		Short:   "Start the gosupervisor daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath := args[0]
			return process.RunServer(cfgPath)
		},
	}

	var cmdStatus = &cobra.Command{
		Use:   "status",
		Short: "Show process status",
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, err := grpc.Dial(serverAddr, grpc.WithInsecure())
			if err != nil {
				return errors.Wrapf(err, "did not connect daemon, addr:%v", serverAddr)
			}
			defer conn.Close()
			c := pb.NewGoSupervisorClient(conn)
			ctx := context.Background()
			r, err := c.List(ctx, &pb.ListRequest{})
			if err != nil {
				return errors.Wrap(err, "call List")
			}
			table := tablewriter.NewWriter(os.Stdout)
			if !verbose {
				table.SetHeader([]string{"ProcessName", "Pid", "Status", "ProcessDesc", "Desc"})
				for _, v := range r.Process {
					table.Append([]string{
						v.Spec.ProcessName,
						fmt.Sprint(v.Status.Pid),
						fmt.Sprint(v.Status.Status),
						v.Status.ProcessDesc,
						v.Spec.Desc,
					})
				}
			} else {
				table.SetHeader([]string{"ProcessName", "Pid", "Status", "ProcessDesc", "Desc", "Directory", "Command", "Environment"})
				for _, v := range r.Process {
					table.Append([]string{
						v.Spec.ProcessName,
						fmt.Sprint(v.Status.Pid),
						fmt.Sprint(v.Status.Status),
						v.Status.ProcessDesc,
						v.Spec.Desc,
						v.Spec.Directory,
						v.Spec.Command,
						fmt.Sprint(v.Spec.Environment),
					})
				}
			}

			table.Render()
			return nil
		},
	}
	cmdStatus.Flags().BoolVarP(&verbose, "verbose", "v", false, "")

	var cmdPing = &cobra.Command{
		Use:   "ping",
		Short: "Check server version",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, err := grpc.Dial(serverAddr, grpc.WithInsecure())
			if err != nil {
				return errors.Wrapf(err, "did not connect daemon, addr:%v", serverAddr)
			}
			defer conn.Close()
			c := pb.NewGoSupervisorClient(conn)
			ctx := context.Background()
			r, err := c.Ping(ctx, &pb.PingRequest{})
			if err != nil {
				return errors.Wrap(err, "call List")
			}
			fmt.Printf("Connection server success, ServiceVersion:%s\n", r.ServiceVersion)
			return nil
		},
	}

	var cmdKill = &cobra.Command{
		Use:   "kill NAME [NAME...]",
		Short: "Kill process",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCmd(pb.CommandRequest_KILL, args)
		},
	}

	var cmdStop = &cobra.Command{
		Use:   "stop NAME [NAME...]",
		Short: "Stop process",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCmd(pb.CommandRequest_STOP, args)
		},
	}

	var cmdStart = &cobra.Command{
		Use:   "start NAME [NAME...]",
		Short: "Start process",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCmd(pb.CommandRequest_START, args)
		},
	}

	var cmdRestart = &cobra.Command{
		Use:   "restart NAME [NAME...]",
		Short: "Restart process",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCmd(pb.CommandRequest_RESTART, args)
		},
	}

	var rootCmd = &cobra.Command{Use: "gosupervisor"}
	rootCmd.AddCommand(cmdDaemon)
	rootCmd.AddCommand(cmdStatus)
	rootCmd.AddCommand(cmdPing)
	rootCmd.AddCommand(cmdKill, cmdStop, cmdStart, cmdRestart)
	rootCmd.Flags().StringVarP(&serverAddr, "server_addr", "s", "127.0.0.1:7766", "daemon listen addr to connect")
	err := rootCmd.Execute()
	if err != nil {
		log.Println("err", err)
	}
}

func runCmd(cmd pb.CommandRequest_Command, args []string) error {
	for _, name := range args {
		conn, err := grpc.Dial(serverAddr, grpc.WithInsecure())
		if err != nil {
			log.Fatalf("did not connect: %v", err)
		}
		defer conn.Close()
		c := pb.NewGoSupervisorClient(conn)
		ctx := context.Background()
		fmt.Printf("exec %v %v\n", cmd.String(), name)
		_, err = c.Command(ctx, &pb.CommandRequest{Command: cmd, ProcessName: name})
		if err != nil {
			return errors.Wrapf(err, "exec %v", cmd.String())
		}
		fmt.Printf("exec %v %v success\n", cmd.String(), name)
	}
	return nil
}
