package tcp_routing_test

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/cloudfoundry-incubator/cf-routing-acceptance-tests/helpers/assets"
	"github.com/cloudfoundry-incubator/cf-routing-test-helpers/helpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Tcp Routing", func() {
	Context("single app port", func() {
		var (
			appName            string
			tcpDropletReceiver = assets.NewAssets().TcpDropletReceiver
			serverId1          string
			externalPort1      uint16
		)

		BeforeEach(func() {
			appName = helpers.GenerateAppName()
			serverId1 = "server1"
			cmd := fmt.Sprintf("tcp-droplet-receiver --serverId=%s", serverId1)
			spaceName := context.RegularUserContext().Space
			externalPort1 = helpers.CreateTcpRouteWithRandomPort(spaceName, domainName, DEFAULT_TIMEOUT)

			// Uses --no-route flag so there is no HTTP route
			helpers.PushAppNoStart(appName, tcpDropletReceiver, routingConfig.GoBuildpackName, domainName, CF_PUSH_TIMEOUT, "-c", cmd, "--no-route")
			helpers.EnableDiego(appName, DEFAULT_TIMEOUT)
			helpers.UpdatePorts(appName, []uint16{3333}, DEFAULT_TIMEOUT)
			helpers.CreateRouteMapping(appName, "", externalPort1, 3333, DEFAULT_TIMEOUT)
			helpers.StartApp(appName, DEFAULT_TIMEOUT)
		})

		AfterEach(func() {
			helpers.AppReport(appName, DEFAULT_TIMEOUT)
			helpers.DeleteApp(appName, DEFAULT_TIMEOUT)
		})

		It("maps a single external port to an application's container port", func() {
			// connect to TCP router/ELB and assert on something
			for _, routerAddr := range routingConfig.Addresses {
				resp, err := sendAndReceive(routerAddr, externalPort1)
				Expect(err).ToNot(HaveOccurred())
				Expect(resp).To(ContainSubstring(serverId1))
			}
		})

		Context("single external port to two different apps", func() {
			var (
				secondAppName string
				serverId2     string
			)

			BeforeEach(func() {
				secondAppName = helpers.GenerateAppName()
				serverId2 = "server2"
				cmd := fmt.Sprintf("tcp-droplet-receiver --serverId=%s", serverId2)

				// Uses --no-route flag so there is no HTTP route
				helpers.PushAppNoStart(secondAppName, tcpDropletReceiver, routingConfig.GoBuildpackName, domainName, CF_PUSH_TIMEOUT, "-c", cmd, "--no-route")
				helpers.EnableDiego(secondAppName, DEFAULT_TIMEOUT)
				helpers.UpdatePorts(secondAppName, []uint16{3333}, DEFAULT_TIMEOUT)
				helpers.CreateRouteMapping(secondAppName, "", externalPort1, 3333, DEFAULT_TIMEOUT)
				helpers.StartApp(secondAppName, DEFAULT_TIMEOUT)
			})

			AfterEach(func() {
				helpers.AppReport(secondAppName, DEFAULT_TIMEOUT)
				helpers.DeleteApp(secondAppName, DEFAULT_TIMEOUT)
			})

			It("maps single external port to both applications", func() {
				// connect to TCP router/ELB and assert on something
				for _, routerAddr := range routingConfig.Addresses {
					actualServerId1, err1 := getServerResponse(routerAddr, externalPort1)
					Expect(err1).ToNot(HaveOccurred())
					actualServerId2, err2 := getServerResponse(routerAddr, externalPort1)
					Expect(err2).ToNot(HaveOccurred())
					expectedServerIds := []string{serverId1, serverId2}
					Expect(expectedServerIds).To(ConsistOf(actualServerId1, actualServerId2))
				}
			})
		})
	})
})

const (
	DEFAULT_CONNECT_TIMEOUT = 5 * time.Second
	CONN_TYPE               = "tcp"
	BUFFER_SIZE             = 1024
)

func getServerResponse(addr string, externalPort uint16) (string, error) {
	response, err := sendAndReceive(addr, externalPort)
	if err != nil {
		return "", err
	}
	tokens := strings.Split(response, ":")
	if len(tokens) == 0 {
		return "", errors.New("Could not extract server id from response")
	}
	return tokens[0], nil
}

func sendAndReceive(addr string, externalPort uint16) (string, error) {
	address := fmt.Sprintf("%s:%d", addr, externalPort)

	conn, err := net.DialTimeout(CONN_TYPE, address, DEFAULT_CONNECT_TIMEOUT)
	if err != nil {
		return "", err
	}

	message := []byte(fmt.Sprintf("Time is %d", time.Now().Nanosecond()))
	_, err = conn.Write(message)
	if err != nil {
		return "", err
	}

	buff := make([]byte, BUFFER_SIZE)
	_, err = conn.Read(buff)
	if err != nil {
		return "", err
	}

	return string(buff), conn.Close()
}