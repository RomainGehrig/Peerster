import sys

ip = sys.argv[1]
rtimer = "-rtimer=300 "

ips = []
for i in range(0, 10):
    ips.append(ip + ":" + str(5000 + i))

peers = [
    [ips[1], ips[3]],
    [ips[0], ips[2]],
    [ips[1], ips[3]]
]

for i in range(0, 4):
    file = open("usr" + str(i) + ".bat", "w")
    file.write("Peerster.exe ")
    file.write("-UIPort=" + str(8080 + i) + " ")
    file.write(rtimer)
    file.write("-gossipAddr=" +  ips[i] + " ")
    file.write("-name=Node" + str(i) + " ")
    file.write("-peers=")
    l = len(peers[i])
    for j in range(l):
        file.write(peers[i][j])
        if j != l-1:
            file.write(",")
