Offer Server
===

* Load products via yeahmobi api into memory
* Receive retrival request and return offers proper using udp protocol


Runtime Environment
---

* golang (see ways of installation as follows)

  * centOS: `yum install golang`

  * ubuntu: `apt-get install golang`

  * macOS: `brew install golang`

  * windows: what the fuck!!!


Dependecy Installation
---

    make deps


Mock
---

    make mock
    bin/tmock

    # see raw ad info:
    curl "http://127.0.0.1:19992/dump?page_num=10&page_size=3"

    # retrieval a rand raw ad
    curl "http://127.0.0.1:19991/getad?platform=Android&country=HK&os=Android&lang=EN&w=300&h=300"


Testing and Benchmark
---

    make test
    make bench
