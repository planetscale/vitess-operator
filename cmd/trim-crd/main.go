/*
trim-crd is a tool to reduce the size of a CRD.

Now that kubebuilder's controller-gen tool embeds OpenAPI schema for core k8s
types that we happen to include in our structs, our CRDs have become so big that
some of them cannot be applied anymore because the apply annotation would exceed
256KiB.

There does not appear to be any way to tell controller-gen not to embed the
schema for core types, so we strip it out after the fact to return to the old
behavior. This means no validation is performed on these embedded types.

We haven't found a reliable way to automatically distinguish core k8s types when
looking at the raw OpenAPI schema, so we use an explicit marker tag on certain
fields to mark them for removal by this tool:

  // +kubebuilder:validation:EmbeddedResource

Note that we are abusing the marker for a purpose other than the one that was
intended. The name just happens to sound about right and we don't otherwise
intend to use the marker for its actual purpose any time soon.
*/
package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

const (
	embeddedResourceTag      = "x-kubernetes-embedded-resource"
	preserveUnknownFieldsTag = "x-kubernetes-preserve-unknown-fields"
)

func main() {
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Requires at least one file name")
		os.Exit(1)
	}

	for _, fileName := range flag.Args() {
		if err := trimFile(fileName); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to trim %v: %v", fileName, err)
			os.Exit(2)
		}
	}
}

func trimFile(fileName string) error {
	data, err := ioutil.ReadFile(fileName)
	if err != nil {
		return fmt.Errorf("can't read file: %v", err)
	}

	obj := &unstructured.Unstructured{}
	if err := yaml.Unmarshal(data, obj); err != nil {
		return fmt.Errorf("can't unmarshal YAML: %v", err)
	}

	if err := trimCRD(obj); err != nil {
		return err
	}

	data, err = yaml.Marshal(obj)
	if err != nil {
		return fmt.Errorf("can't marshal YAML: %v", err)
	}
	if err := ioutil.WriteFile(fileName, data, 0644); err != nil {
		return fmt.Errorf("can't write file: %v", err)
	}
	return nil
}

func trimCRD(crd *unstructured.Unstructured) error {
	if crd.GetKind() != "CustomResourceDefinition" {
		return fmt.Errorf("%q is not a CustomResourceDefinition; found kind: %q", crd.GetName(), crd.GetKind())
	}

	val, found, err := unstructured.NestedFieldNoCopy(crd.Object, "spec", "validation", "openAPIV3Schema", "properties")
	if err != nil || !found {
		return fmt.Errorf("can't find spec.validation.openAPIV3Schema.properties: %v", err)
	}
	properties, ok := val.(map[string]interface{})
	if !ok {
		return fmt.Errorf("spec.validation.openAPIV3Schema.properties is not an object")
	}

	return trimProperties("openAPIV3Schema", properties)
}

func trimProperties(parentName string, properties map[string]interface{}) error {
	for name, val := range properties {
		property, ok := val.(map[string]interface{})
		if !ok {
			return fmt.Errorf("value of property %q is not an object", name)
		}
		if parentName == "dataVolumeClaimTemplate" && name == "dataSource" {
			// We delete the validation for this field because many clients send
			// an invalid values (null) due to a bug in the k8s API (missing
			// omitempty) that existed until k8s 1.14. Even after clients are
			// upgraded to newer k8s client-go versions, this invalid data can
			// persist in CRDs so we need to tolerate it.
			delete(properties, name)
			continue
		}
		if err := trimProperty(name, property); err != nil {
			return fmt.Errorf("can't trim property %q: %v", name, err)
		}
	}
	return nil
}

func trimProperty(propertyName string, property map[string]interface{}) error {
	embedded, found, err := unstructured.NestedBool(property, embeddedResourceTag)
	if err != nil {
		return fmt.Errorf("invalid %v tag: %v", embeddedResourceTag, err)
	}

	if found && embedded {
		// Remove the embedded tag itself.
		delete(property, embeddedResourceTag)

		// If it's an array, process the items.
		if items, ok := property["items"].(map[string]interface{}); ok {
			property = items
		}

		// Remove all sub-properties.
		delete(property, "properties")

		// Tell k8s to accept any value here.
		property[preserveUnknownFieldsTag] = true

		return nil
	}

	// If it's an array, process the items.
	if items, ok := property["items"].(map[string]interface{}); ok {
		property = items
	}

	// Recurse into sub-properties, if any.
	if properties, ok := property["properties"].(map[string]interface{}); ok {
		return trimProperties(propertyName, properties)
	}

	return nil
}
