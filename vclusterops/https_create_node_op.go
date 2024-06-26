/*
 (c) Copyright [2023-2024] Open Text.
 Licensed under the Apache License, Version 2.0 (the "License");
 You may not use this file except in compliance with the License.
 You may obtain a copy of the License at

 http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package vclusterops

import (
	"errors"
	"fmt"

	"github.com/vertica/vcluster/vclusterops/util"
)

type httpsCreateNodeOp struct {
	opBase
	opHTTPSBase
	RequestParams map[string]string
}

func makeHTTPSCreateNodeOp(newNodeHosts []string, bootstrapHost []string,
	useHTTPPassword bool, userName string, httpsPassword *string,
	vdb *VCoordinationDatabase, scName string) (httpsCreateNodeOp, error) {
	op := httpsCreateNodeOp{}
	op.name = "HTTPSCreateNodeOp"
	op.description = "Create node in catalog"
	op.hosts = bootstrapHost
	op.RequestParams = make(map[string]string)
	// HTTPS create node endpoint requires passing everything before node name
	op.RequestParams["catalog-prefix"] = vdb.CatalogPrefix + "/" + vdb.Name
	op.RequestParams["data-prefix"] = vdb.DataPrefix + "/" + vdb.Name
	op.RequestParams["hosts"] = util.ArrayToString(newNodeHosts, ",")
	if scName != "" {
		op.RequestParams["subcluster"] = scName
	}
	err := op.validateAndSetUsernameAndPassword(op.name,
		useHTTPPassword, userName, httpsPassword)

	return op, err
}

func (op *httpsCreateNodeOp) setupClusterHTTPRequest(hosts []string) error {
	for _, host := range hosts {
		httpRequest := hostHTTPRequest{}
		httpRequest.Method = PostMethod
		// note that this will be updated in Prepare()
		// because the endpoint only accept parameters in query
		httpRequest.buildHTTPSEndpoint("nodes")
		if op.useHTTPPassword {
			httpRequest.Password = op.httpsPassword
			httpRequest.Username = op.userName
		}
		httpRequest.QueryParams = op.RequestParams
		op.clusterHTTPRequest.RequestCollection[host] = httpRequest
	}

	return nil
}

func (op *httpsCreateNodeOp) updateQueryParams(execContext *opEngineExecContext) error {
	for _, host := range op.hosts {
		profile, ok := execContext.networkProfiles[host]
		if !ok {
			return fmt.Errorf("[%s] unable to find network profile for host %s", op.name, host)
		}
		op.RequestParams["broadcast"] = profile.Broadcast
	}
	return nil
}

func (op *httpsCreateNodeOp) prepare(execContext *opEngineExecContext) error {
	err := op.updateQueryParams(execContext)
	if err != nil {
		return err
	}

	execContext.dispatcher.setup(op.hosts)

	return op.setupClusterHTTPRequest(op.hosts)
}

func (op *httpsCreateNodeOp) execute(execContext *opEngineExecContext) error {
	if err := op.runExecute(execContext); err != nil {
		return err
	}

	return op.processResult(execContext)
}

func (op *httpsCreateNodeOp) finalize(_ *opEngineExecContext) error {
	return nil
}

type httpsCreateNodeResponse map[string][]map[string]string

func (op *httpsCreateNodeOp) processResult(_ *opEngineExecContext) error {
	var allErrs error

	for host, result := range op.clusterHTTPRequest.ResultCollection {
		op.logResponse(host, result)

		if result.isPassing() {
			// The response object will be a dictionary, an example:
			// {'created_nodes': [{'name': 'v_running_db_node0002', 'catalog_path': '/data/v_running_db_node0002_catalog'},
			//                    {'name': 'v_running_db_node0003', 'catalog_path': '/data/v_running_db_node0003_catalog'}]}
			var responseObj httpsCreateNodeResponse
			err := op.parseAndCheckResponse(host, result.content, &responseObj)

			if err != nil {
				allErrs = errors.Join(allErrs, err)
				continue
			}
			_, ok := responseObj["created_nodes"]
			if !ok {
				err = fmt.Errorf(`[%s] response does not contain field "created_nodes"`, op.name)
				allErrs = errors.Join(allErrs, err)
			}
		} else {
			allErrs = errors.Join(allErrs, result.err)
		}
	}

	return allErrs
}
