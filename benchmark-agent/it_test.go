package main

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/hyperpilotio/container-benchmarks/benchmark-agent/apis"
	testHttp "github.com/hyperpilotio/container-benchmarks/benchmark-agent/test/http"
)

func TestByGinkgo(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Base Suite")
}

var httpClientConfig = testHttp.NewClientConfig()

var _ = Describe("Test creating benchmarks", func() {
	AfterEach(func() {
		testHttp.Response(
			httpClientConfig.NewSling().
				Delete("benchmarks/busycpu"),
		)
	})

	It("creates benchmarks", func() {
		req := &apis.Benchmark{
			Name:  "busycpu",
			Count: 4,
			Resources: &apis.Resources{
				CPUShares: 512,
			},
			Image: "hyperpilot/busycpu",
		}
		resp := testHttp.Response(
			httpClientConfig.NewSling().
				Post("benchmarks").
				BodyJSON(req),
		)

		jsonBody := resp.JSONBody()
		Expect(jsonBody.Get("error").MustBool()).To(BeFalse())
	})
})

var _ = Describe("Test updating benchmarks", func() {
	BeforeEach(func() {
		req := &apis.Benchmark{
			Name:  "busycpu",
			Count: 4,
			Resources: &apis.Resources{
				CPUShares: 512,
			},
			Image: "hyperpilot/busycpu",
		}
		testHttp.Response(
			httpClientConfig.NewSling().
				Post("benchmarks").
				BodyJSON(req),
		)
	})

	AfterEach(func() {
		testHttp.Response(
			httpClientConfig.NewSling().
				Delete("benchmarks/busycpu"),
		)
	})

	It("updates benchmarks", func() {
		req := &apis.Resources{
			CPUShares: 256,
		}
		resp := testHttp.Response(
			httpClientConfig.NewSling().
				Post("benchmarks/busycpu/resources").
				BodyJSON(req),
		)

		jsonBody := resp.JSONBody()
		Expect(jsonBody.Get("error").MustBool()).To(BeFalse())
	})
})

var _ = Describe("Test deleting benchmarks", func() {
	BeforeEach(func() {
		req := &apis.Benchmark{
			Name:  "busycpu",
			Count: 4,
			Resources: &apis.Resources{
				CPUShares: 512,
			},
			Image: "hyperpilot/busycpu",
		}
		testHttp.Response(
			httpClientConfig.NewSling().
				Post("benchmarks").
				BodyJSON(req),
		)
	})

	It("deletes benchmarks", func() {
		resp := testHttp.Response(
			httpClientConfig.NewSling().
				Delete("benchmarks/busycpu"),
		)
		jsonBody := resp.JSONBody()
		Expect(jsonBody.Get("error").MustBool()).To(BeFalse())
	})
})
