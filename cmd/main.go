package main

import (
	"fmt"
	"github.com/ottramst/go-kibana"
)

func main() {

	client, err := kibana.NewBasicAuthClient(
		"elastic",
		"lDb04gDWVk7sF9565L1wV4Q2",
		kibana.WithBaseURL("https://kibana-stg.px.tools"),
	)

	if err != nil {
		fmt.Println(err)
	}

	// Create
	createOpt := &kibana.CreateSpaceOptions{
		ID:          kibana.String("test-space"),
		Name:        kibana.String("Test Space"),
		Description: kibana.String("This is a test space"),
	}
	createdSpace, resp, err := client.Spaces.CreateSpace(createOpt)

	fmt.Println(resp.StatusCode)
	fmt.Println(err)
	fmt.Println(createdSpace)

	// Update
	updateOpt := &kibana.UpdateSpaceOptions{
		Name: kibana.String("Test Space Updated"),
	}
	updatedSpace, resp, err := client.Spaces.UpdateSpace("test-space", updateOpt)

	fmt.Println(resp.StatusCode)
	fmt.Println(err)
	fmt.Println(updatedSpace)

	// Get single
	space, resp, err := client.Spaces.GetSpace("test-space")

	fmt.Println(resp.StatusCode)
	fmt.Println(err)
	fmt.Println(space)

	// Delete single
	resp, err = client.Spaces.DeleteSpace("test-space")

	fmt.Println(resp.StatusCode)
	fmt.Println(err)

	// Get all
	opt := &kibana.GetAllSpacesOptions{}
	spaces, resp, err := client.Spaces.GetAllSpaces(opt)

	fmt.Println(resp.StatusCode)
	fmt.Println(err)

	for _, space := range spaces {
		fmt.Println(space)
	}

}
