Simple Bolt
============

[![Build Status](https://travis-ci.org/xyproto/simplebolt.svg?branch=master)](https://travis-ci.org/xyproto/simplebolt)
[![GoDoc](https://godoc.org/github.com/xyproto/simplebolt?status.svg)](http://godoc.org/github.com/xyproto/simplebolt)
[![Report Card](https://img.shields.io/badge/go_report-A+-brightgreen.svg?style=flat)](http://goreportcard.com/report/xyproto/simplebolt)

Simple way to use the [Bolt](github.com/boltdb/bolt) database. Similar design to [simpleredis](https://github.com/xyproto/simpleredis).


Online API Documentation
------------------------

[godoc.org](http://godoc.org/github.com/xyproto/simplebolt)


Features and limitations
------------------------

* Supports simple use of lists, hashmaps, sets and key/values
* Deals mainly with strings


Example usage
-------------

~~~go
package main

import (
	"log"

	"github.com/xyproto/simplebolt"
)

func main() {
	// New bolt database
	db, err := simplebolt.New("bolt.db")
	if err != nil {
		log.Fatalf("Could not create database! %s", err)
	}
	defer db.Close()

	// Create a list named "greetings"
	list, err := simplebolt.NewList(db, "greetings")
	if err != nil {
		log.Fatalf("Could not create a list! %s", err)
	}

	// Add "hello" to the list
	if err := list.Add("hello"); err != nil {
		log.Fatalf("Could not add an item to the list! %s", err)
	}

	// Get the last item of the list
	if item, err := list.GetLast(); err != nil {
		log.Fatalf("Could not fetch the last item from the list! %s", err)
	} else {
		log.Println("The value of the stored item is:", item)
	}

	// Remove the list
	if err := list.Remove(); err != nil {
		log.Fatalf("Could not remove the list! %s", err)
	}
}
~~~

Version, license and author
---------------------------

* License: MIT
* API version: 3.0
* Author: Alexander F Rødseth <xyproto@archlinux.org>

