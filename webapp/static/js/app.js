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
    const msgText = document.getElementById("messageText");
    msgText.addEventListener("keyup", onEnter(clickSendMessage));
    const nodeAddr = document.getElementById("nodeValue");
    nodeAddr.addEventListener("keyup", onEnter(clickNewNode));

    refreshInfo();
    setInterval(refreshInfo, 3000);
}

function refreshInfo() {
    retrieveMessages();
    retrieveNodes();
    retrievePeerID();
}

function compareStrings(s1, s2) {
    let as = s1.toLowerCase();
    let bs = s2.toLowerCase();
    if (as < bs) return -1;
    if (as > bs) return 1;
    return 0;
}

function sortMessages(msgs) {
    if (msgs === null)
        return [];

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
    if (nodes === null)
        return [];

    nodes.sort();
    return nodes;
}

function onEnter(func) {
    return function(event) {
        event.preventDefault();
        if (event.keyCode === 13) {
            func();
        }
    };
}

function clickSendMessage() {
    const elem = document.getElementById("messageText");
    const text = elem.value;
    // Reset text
    elem.value = "";
    if (text !== "") {
        sendMessage(text);
    }
    refreshInfo();
}

function clickNewNode() {
    const elem = document.getElementById("nodeValue");
    const addr = elem.value;

    if (nodeAddrLooksValid(addr)) {
        addNode(addr);
        refreshInfo();
    } else {
        $("#nodeError").fadeIn();
        setTimeout(() => $("#nodeError").fadeOut(), 3000);
    }

    elem.value = "";
}

function nodeAddrLooksValid(str) {
    const splitted = str.split(":");
    if (splitted.length !== 2)
        return false;

    const parsed = Number(splitted[1]);
    // Is not a number
    if (parsed === NaN)
        return false;

    // Is not an integer
    if (~~parsed !== parsed)
        return false;

    return true;
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
