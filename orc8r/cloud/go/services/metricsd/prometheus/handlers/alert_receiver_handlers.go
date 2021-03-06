/*
 * Copyright (c) Facebook, Inc. and its affiliates.
 * All rights reserved.
 *
 * This source code is licensed under the BSD-style license found in the
 * LICENSE file in the root directory of this source tree.
 */

package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"magma/orc8r/cloud/go/obsidian/handlers"
	"magma/orc8r/cloud/go/services/metricsd/prometheus/alerting/receivers"

	"github.com/labstack/echo"
	"github.com/prometheus/alertmanager/config"
)

func GetConfigureAlertReceiverHandler(webServerURL string) func(c echo.Context) error {
	return func(c echo.Context) error {
		networkID, nerr := handlers.GetNetworkId(c)
		if nerr != nil {
			return nerr
		}
		url := makeNetworkReceiverPath(webServerURL, networkID)
		return configureAlertReceiver(c, url)
	}
}

func GetRetrieveAlertReceiverHandler(webServerURL string) func(c echo.Context) error {
	return func(c echo.Context) error {
		networkID, nerr := handlers.GetNetworkId(c)
		if nerr != nil {
			return nerr
		}
		url := makeNetworkReceiverPath(webServerURL, networkID)
		return retrieveAlertReceivers(c, url)
	}
}

func GetRetrieveAlertRouteHandler(webServerURL string) func(c echo.Context) error {
	return func(c echo.Context) error {
		networkID, nerr := handlers.GetNetworkId(c)
		if nerr != nil {
			return nerr
		}
		url := makeNetworkRoutePath(webServerURL, networkID)
		return retrieveAlertRoute(c, url)
	}
}

func GetUpdateAlertRouteHandler(webServerURL string) func(c echo.Context) error {
	return func(c echo.Context) error {
		networkID, nerr := handlers.GetNetworkId(c)
		if nerr != nil {
			return nerr
		}
		url := makeNetworkRoutePath(webServerURL, networkID)
		return updateAlertRoute(c, url)
	}
}

func configureAlertReceiver(c echo.Context, url string) error {
	receiver, err := buildReceiverFromContext(c)
	if err != nil {
		return handlers.HttpError(err, http.StatusInternalServerError)
	}

	err = sendConfig(receiver, url)
	if err != nil {
		return handlers.HttpError(err, http.StatusInternalServerError)
	}
	return c.NoContent(http.StatusOK)
}

func retrieveAlertReceivers(c echo.Context, url string) error {
	client := &http.Client{}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return handlers.HttpError(fmt.Errorf("alert server responded with error"), resp.StatusCode)
	}
	var recs []receivers.Receiver
	err = json.NewDecoder(resp.Body).Decode(&recs)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, fmt.Errorf("error decoding server response %v", err))
	}
	return c.JSON(http.StatusOK, recs)
}

func retrieveAlertRoute(c echo.Context, url string) error {
	client := &http.Client{}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var route config.Route
	err = json.NewDecoder(resp.Body).Decode(&route)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, fmt.Errorf("error decoding server response %v", err))
	}
	return c.JSON(http.StatusOK, route)
}

func updateAlertRoute(c echo.Context, url string) error {
	route, err := buildRouteFromContext(c)
	if err != nil {
		return err
	}
	client := &http.Client{}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	err = sendConfig(route, url)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, err)
	}
	return c.NoContent(http.StatusOK)
}

func buildReceiverFromContext(c echo.Context) (receivers.Receiver, error) {
	wrapper := receivers.Receiver{}
	err := json.NewDecoder(c.Request().Body).Decode(&wrapper)
	if err != nil {
		return receivers.Receiver{}, err
	}
	return wrapper, nil
}

func buildRouteFromContext(c echo.Context) (config.Route, error) {
	route := config.Route{}
	err := json.NewDecoder(c.Request().Body).Decode(&route)
	if err != nil {
		return config.Route{}, err
	}
	return route, nil
}

func makeNetworkReceiverPath(webServerURL, networkID string) string {
	return webServerURL + "/" + networkID + "/receiver"
}

func makeNetworkRoutePath(webSeverURL, networkID string) string {
	return webSeverURL + "/" + networkID + "/receiver/route"
}
