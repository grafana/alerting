import axios from "axios"
import pAll from "p-all"
import { randomUUID } from "crypto"
import inquirer from "inquirer"

const HTTP_REQUEST_CONCURRENCY = 5 // you can play around with this if you like, but too many concurrent requests can lock the database
const INTERAL_SECONDS = 28800 // 8h
const ORG_ID = 1

const answers = await inquirer.prompt([
  { type: 'input', name: 'FOLDER_UID', message: 'Specify the folder_uid to store the rules in' },
  { type: 'input', name: 'RULE_GROUP', message: 'Specify the group name', default: 'lots-of-rules' },
  { type: 'number', name: 'NUM_RULES', message: 'How many rules?', default: 5000 }
])

const makeActions = (answers) => new Array(answers.NUM_RULES).fill(0).map((_, index) => {
  return async () => {
    console.log('Request #' + index)
    await makeRequest(answers)
    console.log('Request #' + index + ': done')
  }
})

await pAll(makeActions(answers), {
  concurrency: HTTP_REQUEST_CONCURRENCY,
})

console.log('import done')

async function makeRequest(answers) {
  await axios({
    "method": "POST",
    "url": "http://localhost:3000/api/v1/provisioning/alert-rules/",
    "headers": {
      "Authorization": "Basic YWRtaW46YWRtaW4=",
      "Accept": "application/json",
      "Content-Type": "application/json"
    },
    "auth": {
      "username": "admin",
      "password": "admin"
    },
    "data": makeBody(answers)
  })
}

function makeBody () {
  const { FOLDER_UID, RULE_GROUP, NUM_RULES } = answers

  const nonce = randomUUID()

  return {
    "folderUID": FOLDER_UID,
    "condition": "A",
    "interval_seconds": INTERAL_SECONDS,
    "orgID": ORG_ID,
    "ruleGroup": RULE_GROUP,
    "title": `Rule ${nonce}`,
    "execErrState": "Error",
    "data": [
      {
        "relativeTimeRange": {
          "to": 0,
          "from": 0
        },
        "model": {
          "datasource": {
            "type": "__expr__",
            "uid": "__expr__"
          },
          "expression": "1 == 0",
          "conditions": [
            {
              "query": {
                "params": []
              },
              "reducer": {
                "type": "avg",
                "params": [
                  0,
                  0
                ]
              },
              "operator": {
                "type": "and",
                "params": []
              },
              "type": "query",
              "evaluator": {
                "type": "gt",
                "params": []
              }
            }
          ],
          "hide": false,
          "refId": "A",
          "type": "math",
          "intervalMs": 1000,
          "maxDataPoints": 43200
        },
        "datasourceUid": "-100",
        "queryType": "",
        "refId": "A"
      }
    ],
    "for": "8h",
    "noDataState": "NoData"
  }
}
