# OpenSY Mining - Quick Start Guide

## 🚀 البدء السريع

### المتطلبات
- Go 1.21+
- Docker (للـ Pool فقط)
- GCC/Clang (لبناء RandomX)

### البناء
```bash
# بناء كل شيء
make all

# أو خطوة بخطوة
make randomx  # بناء مكتبة RandomX
make build    # بناء البرامج
```

---

## ⛏️ الخيار 1: CoopMine (تجميع أجهزتك)

**الأفضل لـ:** تجميع hashrate من عدة أجهزة شخصية

### على الجهاز الرئيسي:
```bash
WALLET=SYxxxxxx ./scripts/run-coopmine.sh coordinator
```

### على الأجهزة الأخرى:
```bash
COORDINATOR=192.168.1.100:5555 ./scripts/run-coopmine.sh worker
```

---

## 🏊 الخيار 2: Mining Pool (بركة تعدين)

**الأفضل لـ:** تشغيل pool لعدة miners مختلفين

### تشغيل Pool:
```bash
./scripts/run-pool.sh start
```

### توصيل XMRig:
```bash
xmrig -o <POOL_IP>:3333 -u <WALLET> -p worker1 -a rx/0
```

---

## 📊 الواجهات

| الخدمة | العنوان |
|--------|---------|
| Stratum Pool | `tcp://localhost:3333` |
| Pool API | `http://localhost:8080/api/stats` |
| Pool Metrics | `http://localhost:8080/metrics` |
| Pool Health | `http://localhost:8080/health` |
| CoopMine gRPC | `localhost:5555` |

---

## 🔧 الأوامر المفيدة

```bash
# حالة Docker
./scripts/run-pool.sh status

# عرض المساعدة
make help

# تنظيف كل شيء
make clean
./scripts/run-pool.sh stop
```

---

## 📖 مزيد من التفاصيل

- [README.md](README.md) - التوثيق الكامل
- [docs/POOL_OPERATOR.md](docs/POOL_OPERATOR.md) - دليل مشغل الـ Pool
- [docs/COOPMINE_SETUP.md](docs/COOPMINE_SETUP.md) - إعداد CoopMine
