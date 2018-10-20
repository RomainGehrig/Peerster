Vue.component('message', {
    template: `<div>
                <div class="float-right"><strong>{{ message.origin }} - #{{ message.id }}</strong></div>
                {{ message.text }}
              </div>`,
    props: ["message"]
});

let vue = new Vue({
    el: "#vuejs",
    data: {
        messages: [],
        nodes: [],
        peerID: ""
    }
});

// TODO Callback on addNode and sendMessage (or just reload the page)
function init() {
    retrievePeerID();
    refreshInfo();
    setInterval(refreshInfo, 3000);
}

function refreshInfo() {
    retrieveMessages();
    retrieveNodes();
}

function compareStrings(s1, s2) {
    let as = s1.toLowerCase();
    let bs = s2.toLowerCase();
    if (as < bs) return -1;
    if (as > bs) return 1;
    return 0;
}

function sortMessages(msgs) {
    msgs.sort((a, b) => {
        if (a["origin"] == b["origin"])
            return a["id"] - b["id"];
        else {
            return compareStrings(a["origin"], b["origin"]);
        }
    });

    return msgs;
}

function sortNodes(nodes) {
    nodes.sort();
    return nodes;
}

// Messages
async function sendMessage(msg) {
    return jsonPost("/message", JSON.stringify({"text": msg}));
}

async function retrieveMessages() {
    jsonGet("/message").then((obj) => vue.messages = sortMessages(obj["messages"]));
}

// ID
async function retrievePeerID() {
    jsonGet("/id").then((obj) => vue.peerID = obj["id"]);
}

// Nodes
async function retrieveNodes() {
    jsonGet("/node").then((obj) => vue.nodes = sortNodes(obj["nodes"]));
}
async function addNode(node) {
    return jsonPost("/node", JSON.stringify({"addr": node}));
}

async function jsonQuery(method, endpoint, body) {
    return fetch(endpoint, {
        method: method,
        mode: 'cors',
        body: body,
        cache: "no-cache",
        headers: {
            "Accept": "application/json",
            "Content-Type": "application/json"
        }
    }).then((resp) => resp.json());
}

async function jsonGet(endpoint) {
    return jsonQuery("GET", endpoint, null);
}

async function jsonPost(endpoint, jsonData) {
    return jsonQuery("POST", endpoint, jsonData);
}
