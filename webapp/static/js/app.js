
let vue = new Vue({
    el: "#vuejs",
    data: {
        messages: [],
        nodes: [],
        peerID: "NodeA"
    }
});

// TODO Callback on addNode and sendMessage (or just reload the page)

async function retrieveMessages() {
    jsonGet("/message").then((obj) => vue.messages = obj["messages"]);
}

async function addNode(node) {
    return jsonPost("/node", JSON.stringify(node));
}

async function sendMessage(msg) {
    return jsonPost("/message", JSON.stringify(msg));
}

async function retrieveNodes() {
    jsonGet("/node").then((obj) => vue.nodes = obj["nodes"]);
}

async function retrievePeerID() {
    jsonGet("/id").then((obj) => vue.id = obj["id"]);
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
