Vue.component('message', {
    template: `<div>
                <div class="float-right"><strong>{{ message.origin }} - #{{ message.id }}</strong></div>
                {{ message.text }}
              </div>`,
    props: ["message"]
});

Vue.component('tab-content', {
    template: ` <div role="tabpanel" :id="destination" class="tab-pane" :class="{ 'active': active, show: active }">
                    <message v-for="(message, index) in messages" v-bind:message="message"></message>
                    <div class="hr-line-dashed"></div>
                    <div class="input-group">
                        <input type="text" id="messageText" class="form-control" placeholder="Type message...">
                        <span class="input-group-append"> <button type="button" class="btn btn-primary" onclick="clickSendMessage()">Send message to {{destination}}</button> </span>
                    </div>
                </div>`,
    props: ["destination", "messages", "active"]
});

const RUMOR_TAB = "_rumors";

let vue = new Vue({
    el: "#vuejs",
    data: {
        // messages: [],
        nodes: [],
        peerID: "",
        origins: [],
        activeTab: RUMOR_TAB,
        tabs: { [RUMOR_TAB]: [], node1: [{id: 1, origin: "Dog", text: "Hello hooman"}] }
    }
});

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
    retrievePrivateMessages();
    retrieveNodes();
    retrievePeerID();
    retrieveOrigins();
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

function wantToSendMessageTo(hash) {
    changeTab(hash);

    const dest = stripHash(hash, RUMOR_TAB);
    if (!(dest in vue.tabs)) {
        Vue.set(vue.tabs, dest, []);
    }
}

function stripHash(hash, defaultValue) {
    if (hash === "") {
        return defaultValue;
    } else {
        return hash.slice(1);
    }
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

function changeTab(hash) {
    vue.activeTab = stripHash(hash, RUMOR_TAB);
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
    jsonGet("/message").then((obj) => Vue.set(vue.tabs, RUMOR_TAB, sortMessages(obj["messages"])));
}

async function retrievePrivateMessages() {
    jsonGet("/pmessage").then((obj) => {
        for (origin in obj) {
            Vue.set(vue.tabs, origin, obj[origin]);
        }
    });
}

// ID
async function retrievePeerID() {
    jsonGet("/id").then((obj) => vue.peerID = obj["id"]);
}

// Origins
async function retrieveOrigins() {
    jsonGet("/origins").then((obj) => vue.origins = sortNodes(obj["origins"]));
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
