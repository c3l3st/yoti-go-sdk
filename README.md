# Yoti Go SDK

[![Build Status](https://github.com/getyoti/yoti-go-sdk/workflows/Unit%20Tests/badge.svg?branch=master)](https://github.com/getyoti/yoti-go-sdk/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/getyoti/yoti-go-sdk)](https://goreportcard.com/report/github.com/getyoti/yoti-go-sdk)
[![PkgGoDev](https://pkg.go.dev/badge/github.com/getyoti/yoti-go-sdk/v3)](https://pkg.go.dev/github.com/getyoti/yoti-go-sdk/v3?tab=doc)
[![License](http://img.shields.io/badge/license-mit-blue.svg?style=flat-square)](https://github.com/getyoti/yoti-go-sdk/blob/master/LICENSE.md)
[![Coverage](https://sonarcloud.io/api/project_badges/measure?project=getyoti%3Ago&metric=coverage)](https://sonarcloud.io/dashboard?id=getyoti%3Ago)
[![Bugs](https://sonarcloud.io/api/project_badges/measure?project=getyoti%3Ago&metric=bugs)](https://sonarcloud.io/dashboard?id=getyoti%3Ago)
[![Code Smells](https://sonarcloud.io/api/project_badges/measure?project=getyoti%3Ago&metric=code_smells)](https://sonarcloud.io/dashboard?id=getyoti%3Ago)
[![Vulnerabilities](https://sonarcloud.io/api/project_badges/measure?project=getyoti%3Ago&metric=vulnerabilities)](https://sonarcloud.io/dashboard?id=getyoti%3Ago)

Welcome to the Yoti Go SDK. This repo contains the tools and step by step instructions you need to quickly integrate your Go back-end with Yoti so that your users can share their identity details with your application in a secure and trusted way.

## Table of Contents

1) [Requirements](#requirements) -
Requirements to use the SDK

1) [Enabling the SDK](#enabling-the-sdk) -
How to add the SDK to your project

1) [Setup](#setup) -
Setup required before using the Yoti services

1) [Products](#products) -
Links to more information about the products offered by the Yoti SDK

1) [Support](#support) -
Please feel free to reach out

## Requirements

Supported Go Versions:
- See [./.github/workflows/test.yaml](./.github/workflows/test.yaml) for supported versions

## Enabling the SDK

Simply add `github.com/getyoti/yoti-go-sdk/v3` as an import:
```Go
import "github.com/getyoti/yoti-go-sdk/v3"
```
or add the following line to your go.mod file (check https://github.com/getyoti/yoti-go-sdk/releases for the latest version)
```

require github.com/getyoti/yoti-go-sdk/v3 v3.11.0
```

## Setup

For each service you will need:

* Your Client SDK ID, generated by [Yoti Hub](https://hub.yoti.com) when you create (and then publish) your app.
* Your .pem file. This is your own unique private key which your browser generates from the [Yoti Hub](https://hub.yoti.com) when you create an application.

## Products

- [Yoti app integration](_docs/PROFILE.md) - Connect with already-verified customers.
  - [Yoti app sandbox](_docs/PROFILE_SANDBOX.md) - Use the Sandbox SDK in conjunction with the Yoti app integration.

## Support

For any questions or support please email [clientsupport@yoti.com](mailto:clientsupport@yoti.com).
Please provide the following to get you up and working as quickly as possible:

* Computer type
* OS version
* Version of Go being used
* Screenshot

Once we have answered your question we may contact you again to discuss Yoti products and services. If you’d prefer us not to do this, please let us know when you e-mail.
