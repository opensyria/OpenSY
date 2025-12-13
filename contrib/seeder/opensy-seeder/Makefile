CXXFLAGS = -O3 -g0
LDFLAGS = $(CXXFLAGS)

# Detect macOS and add OpenSSL/Boost paths if needed
UNAME_S := $(shell uname -s)
ifeq ($(UNAME_S),Darwin)
    # macOS with Homebrew
    CXXFLAGS += -I/opt/homebrew/opt/openssl/include -I/usr/local/opt/openssl/include
    CXXFLAGS += -I/opt/homebrew/include -I/usr/local/include
    LDFLAGS += -L/opt/homebrew/opt/openssl/lib -L/usr/local/opt/openssl/lib
    LDFLAGS += -L/opt/homebrew/lib -L/usr/local/lib
endif

dnsseed: dns.o opensy.o netbase.o protocol.o db.o main.o util.o
	g++ -pthread $(LDFLAGS) -o dnsseed dns.o opensy.o netbase.o protocol.o db.o main.o util.o -lcrypto

%.o: %.cpp *.h
	g++ -std=c++11 -pthread $(CXXFLAGS) -Wall -Wno-unused -Wno-sign-compare -Wno-reorder -Wno-comment -c -o $@ $<

clean:
	rm -f dnsseed *.o

.PHONY: clean
