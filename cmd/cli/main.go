package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"

	flags "github.com/jessevdk/go-flags"
)

type DataFlag struct {
	Key   string `short:"k" long:"key" description:"Key for (Key, Value) pair"`
	Value string `short:"v" long:"value" description:"Value or (Key, Value) pair"`
}

type ClientFlag struct {
	MasterAddress string `short:"m" long:"master" description:"master flag works as both adding new master and specifying target master of adding slave"`
	SlaveAddress  string `short:"s" long:"slave" description:"If slave to be added, master flag must also be passed with specific address"`
}

type ClusterFlag struct {
	Host        string `short:"h" long:"host" description:"register each others' ip addresses - current host and passed host"`
	Startpoint  bool   `long:"startpoint" description:"register each others' ip addresses - current host and passed host"`
	Handshake   bool   `long:"handshake" description:"register each others' ip addresses - current host and passed host"`
	Start       bool   `long:"start" description:"start cluster with registered hosts"`
	PrintLeader bool   `long:"leader" description:"start cluster with registered hosts"`
	PrintNodes  bool   `long:"nodes" description:"start cluster with registered hosts"`
}

const (
	/* constants for "index" of  */
	commandIdx = iota
	keyIdx
	valueIdx
)

const (
	Help    = "help"
	Get     = "get"
	Set     = "set"
	Add     = "add"
	Ls      = "ls"
	List    = "list"
	Exit    = "exit"
	Quit    = "quit"
	Cluster = "cluster"
)

var baseUrl = os.Getenv("CLUSTER_SEVER_URL")

func main() {

	stdReader := bufio.NewReader(os.Stdin)
	if baseUrl == "" {
		baseUrl = "http://localhost:8001"
	}

	for {

		fmt.Print("hash-interface > ")
		inputText, err := stdReader.ReadString('\n')
		if err != nil {
			log.Fatal(err)
		}

		words := strings.Fields(inputText)

		if len(words) == 0 {
			continue
		}

		switch words[commandIdx] {
		case Help:
			// print Instructions
			printInstructions()
			break

		case Get:

			dataFlags := DataFlag{}
			if err := parseGetFlags(&dataFlags, words); err != nil {
				fmt.Println(err)
				continue
			}

			if err := requestGetToServer(dataFlags.Key); err != nil {
				fmt.Println(err)
				continue
			}

			break

		case Set:

			dataFlags := DataFlag{}
			if err := parseSetFlags(&dataFlags, words); err != nil {
				fmt.Println(err)
				continue
			}

			if err := requestSetToServer(dataFlags); err != nil {
				fmt.Println(err)
				continue
			}

			break

		case Add:
			clientFlags, err := parseClientFlags(words)
			if err != nil {
				fmt.Println(err)
				continue
			}

			if err := requestAddClientToServer(clientFlags); err != nil {
				fmt.Println(err)
				continue
			}

			break

		case List, Ls:
			if err := requestClientListToServer(); err != nil {
				fmt.Println(err)
				continue
			}

			break

		case Cluster:
			clusterFlags := ClusterFlag{}
			err := parseClusterFlag(&clusterFlags, words)
			if err != nil {
				fmt.Println(err)
				continue
			}

			if clusterFlags.Host != "" {
				err := requestNodeRegistration(clusterFlags)
				if err != nil {
					fmt.Println(err)
					continue
				}

			} else if clusterFlags.Start {
				err := requestClusterStart()
				if err != nil {
					fmt.Println(err)
					continue
				}

			} else if clusterFlags.PrintLeader {

				err := printLeader()
				if err != nil {
					fmt.Println(err)
					continue
				}

			} else if clusterFlags.PrintNodes {
				err := printRegisteredNodes()
				if err != nil {
					fmt.Println(err)
					continue
				}
			}

			break

		case Exit, Quit:
			return

		}
	}
}

func printInstructions() {
	fmt.Println("Usage :")
	fmt.Println("[COMMANDS] [OPTIONS] [OPTIONS]")
	fmt.Println(" ")
	fmt.Println("Application Commands : ")
	fmt.Println("get 		Retreieve stored value with passed key")
	fmt.Println("set 		Store key and value")
	fmt.Println("add 		Add new Redis client node (master / slave)")
	fmt.Println("list/ls 	Print current registered Redis master, slave clients list")
	fmt.Println("exit/quit 	Exit cli")
	fmt.Println(" ")
	fmt.Println("get/set Options : ")
	fmt.Println("-k, --key= 	key of (key, value) pair to save(set) or retreive(get)")
	fmt.Println("-v, --value= 	value of (key, value) pair to save(set)")
	fmt.Println(" 				(ex. set -k foo -v bar / get -k foo )")
	fmt.Println("add Options : ")
	fmt.Println("-m, --master= 	new Redis node address")
	fmt.Println("				Used for specifying existing Master client,")
	fmt.Println("				if 'slave' flag is set)")
	fmt.Println("-s, --slave= 	new Redis Slave node address")
	fmt.Println("				'master' flag must be set to specify new slave's master")

}

func parseClientFlags(words []string) (ClientFlag, error) {

	clientFlags := ClientFlag{}
	if _, err := flags.ParseArgs(&clientFlags, words); err != nil {
		return ClientFlag{}, err
	}

	if clientFlags.MasterAddress == "" {
		if clientFlags.SlaveAddress == "" {
			return ClientFlag{}, fmt.Errorf("Flags must be set for Client Addition")
		}
		return ClientFlag{}, fmt.Errorf("Master Flag must be presented")
	}

	return clientFlags, nil
}

func parseGetFlags(dataFlags *DataFlag, words []string) error {

	if _, err := flags.ParseArgs(dataFlags, words); err != nil {
		return err
	}

	if dataFlags.Value != "" {
		return fmt.Errorf("Get command cannot have 'Value' flag")
	}

	if dataFlags.Key == "" {
		return fmt.Errorf("Get command must have 'Key' flag")
	}

	return nil
}

func parseSetFlags(dataFlags *DataFlag, words []string) error {

	if _, err := flags.ParseArgs(dataFlags, words); err != nil {
		return err
	}

	if dataFlags.Value == "" {
		return fmt.Errorf("Set command must have 'Value' flag")
	}

	if dataFlags.Key == "" {
		return fmt.Errorf("Set command must have 'Key' flag")
	}

	return nil
}

func parseClusterFlag(clusterFlag *ClusterFlag, words []string) error {

	if _, err := flags.ParseArgs(clusterFlag, words); err != nil {
		return err
	}

	if !((clusterFlag.Host == "") != (clusterFlag.Start == false)) {
		return fmt.Errorf("Cluster command must have only one flag set")
	}

	return nil
}
