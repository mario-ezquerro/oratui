---
title: "Cómo construí ora-tui: Un gestor en consola (TUI) en Go sin dependencias para Oracle RAC y Single Instance"
published: false
description: "Descubre cómo diseñar una interfaz TUI en Go con marcos Unicode (┌─┐ / ╔═╗), auto-descubrimiento de clusters Oracle RAC y flujos de mantenimiento guiados por flechas sin depender de Oracle Instant Client."
tags: go, oracle, linux, database
---

Son las 3:15 AM de una guardia de producción. Te conectas por SSH a un nodo de **Oracle RAC** en Oracle Linux porque ha saltado una alerta crítica. En cuestión de segundos tienes que:

1. Comprobar si los demonios del cluster (`CRS`, `CSSD`, `EVMD`, `ASM`) están `ONLINE` en todos los nodos (`crsctl check cluster -all` y `crsctl status resource -t`).
2. Cambiar del usuario `grid` al usuario `oracle` para abrir `sqlplus / as sysdba`.
3. Averiguar qué sesión está bloqueando una tabla crítica en `gv$session` (`enq: TX - row lock contention`) o qué tablespace acaba de lanzar un `ORA-01654` porque llegó al 93% sin `AUTOEXTEND`.
4. Si toca mantenimiento de sistema operativo en un nodo, reubicar todos los servicios RAC (`srvctl relocate service`), parar la instancia local y detener la pila CRS sin provocar ni un segundo de caída en las aplicaciones.

Después de repetir este ritual cientos de veces, decidí construir **`ora-tui`**: una herramienta interactiva de terminal (**TUI**) escrita en **Go**, empaquetada como un único binario estático (`CGO_ENABLED=0`) e instalable vía `dnf install ora-tui`.

En este artículo te cuento cómo funciona por dentro, cómo resolví los retos de arquitectura en servidores Oracle y cómo puedes probar su modo simulador (`--mock`) en tu propio portátil.

---

## 🖥️ El Diseño: Marcos Unicode (`┌─┐` y `╔═╗`) y Navegación con Flechas

Uno de los objetivos principales era que la herramienta se sintiera rápida y clara sobre cualquier sesión SSH, utilizando una jerarquía visual de **marcos Unicode**:

* **Borde Simple (`┌ ─ ┐ │ └ ┘`)** para los paneles secundarios y tablas de datos.
* **Borde Doble (`╔ ═ ╗ ║ ╚ ╝`)** para la cabecera principal, el panel que tiene el foco activo, las **ventanas emergentes de mantenimiento** y los diálogos de confirmación.
* **Barras de progreso por bloques (`[█████████░] 93.0%`)** coloreadas según criticidad (verde $<80\%$, amarillo $80\text{–}90\%$, rojo $>90\%$).

Así se ve `ora-tui` en una sesión de terminal:

```text
╔══════════════════════════════════════════════════════════════════════════════════════════════════════════════════════╗
║                              ══ ORA-TUI :: ORACLE CLUSTER & DATABASE MANAGER ══                                      ║
╠══════════════════════════════════════════════════════════════════════════════════════════════════════════════════════╣
║ Modo: [ORACLE RAC] [MOCK DESIGN] │ Host: ora-rac01.corp.local │ Cluster: crs-prod-cluster   │ Ver: 19.24.0.0.0       ║
║ ORACLE_HOME: /u01/app/oracle/product/19.0.0/dbhome_1  │ GRID_HOME: /u01/app/19.0.0/grid                              ║
║ Estado: 2 Inst OPEN │ 4 PDBs │ 3 Sesiones (1 Lock TX) │ 1 Tablespace >90% │ 3 Inválidos │ 1 Alerta Crítica ORA-01654 ║
╠═════════════════════════════╦════════════════════════════════════════════════════════════════════════════════════════╣
║ MENU [Flechas ↑↓ + Enter]   ║ PANTALLA ACTIVA: VISTA INTEGRAL + VENTANA EMERGENTE (Borde Simple ┌─┐ y Doble ╔═╗)     ║
╠═════════════════════════════╬════════════════════════════════════════════════════════════════════════════════════════╣
║ ▸ 1. Cluster & DB Estado    ║ ┌─ NODOS DEL CLUSTER & INSTANCIAS (ORACLE RAC / SINGLE) ─────────────────────────────┐ ║
║   2. Usuarios Conectados    ║ │ NODO       ESTADO   CRS     CSSD    ASM            INST      MODO        FRA_USO   │ ║
║   3. Tablespaces & Files    ║ │ ora-rac01  ONLINE   ONLINE  ONLINE  ONLINE(+ASM1)  PRODCDB1  READ WRITE  [████░░░░]│ ║
║   4. Tablas & Objetos       ║ │ ora-rac02  ONLINE   ONLINE  ONLINE  ONLINE(+ASM2)  PRODCDB2  READ WRITE  [████░░░░]│ ║
║   5. ASM & Red Listeners    ║ └────────────────────────────────────────────────────────────────────────────────────┘ ║
║   6. RMAN, Flashback & DG   ║ ┌─ USUARIOS CONECTADOS (gv$session) ───────────┬─ TABLESPACES & BACKUPS RMAN ────────┐ ║
║   7. Alert Log & Errores    ║ │ SID,SER#  USUARIO    ESTADO   EVENTO / LOCK  │ RECURSO       ESTADO / USO %        │ ║
║   8. Centro Mantenimiento   ║ │ 142,48219 ERP_APP    ACTIVE   db file seq rd │ ERP_DATA_TBS  [█████████░] 93.0% !! │ ║
║   b. Cambiar Marcos ┌─┐/╔═╗ ║ │ 289,11042 CRM_BATCH  ACTIVE   enq: TX lock   │ SYSAUX        [████████░░] 84.4%    │ ║
║   t. Cambiar Topologia      ║ │ 512,9931  CRM_ADMIN  INACT    BLOCKER (289)  │ RMAN INCR L0  COMPLETED (498.4 GB)  │ ║
║   q. Salir (Quit)           ║ │ 604,33105 BI_REPORT  ACTIVE   ON CPU (512MB) │ GRP_RELEASE   GUARANTEED (18.4 GB)  │ ║
║                             ║ └──────────────────────────────────────────────┴─────────────────────────────────────┘ ║
║                             ║                                                                                        ║
║                             ║      ╔══════════════════════════════════════════════════════════════════════════╗      ║
║                             ║      ║      ══ VENTANA DE MANTENIMIENTO [Seleccione con Flechas ↑↓ + Enter] ══  ║      ║
║                             ║      ╠══════════════════════════════════════════════════════════════════════════╣      ║
║                             ║      ║ ▸ [1] Drenar / Aislar Nodo Cluster (Relocate Services + Stop CRS)        ║      ║
║                             ║      ║   [2] Parar / Arrancar Instancia o Base de Datos (srvctl / sqlplus)      ║      ║
║                             ║      ║   [3] Ampliar Tablespace Crítico (+Datafile 32G a ERP_DATA_TBS 93%)      ║      ║
║                             ║      ║   [4] Matar Sesión Bloqueante (KILL SESSION '512,9931,@2' IMMEDIATE)     ║      ║
║                             ║      ║   [5] Actualizar Estadísticas (DBMS_STATS) & Shrink Tablas Online        ║      ║
║                             ║      ║   [6] Recompilar Objetos Inválidos (UTL_RECOMP / COMPILE_SCHEMA)         ║      ║
║                             ║      ║   [7] Backup RMAN Archivelog + Crear Punto Restauración Garantizado      ║      ║
║                             ║      ║   [8] Analizar Errores ORA- en Alert Log & Purgar Trazas ADRCI           ║      ║
║                             ║      ║   [Esc] Cancelar / Cerrar Ventana                                        ║      ║
║                             ║      ╚══════════════════════════════════════════════════════════════════════════╝      ║
╠═════════════════════════════╩════════════════════════════════════════════════════════════════════════════════════════╣
║ ┌─ CONSOLA DE EJECUCION & AUDITORIA EN VIVO ───────────────────────────────────────────────────────────────────────┐ ║
║ │ [08:05:01] Auto-Discovery: Modo=ORACLE RAC │ Cluster=crs-prod-cluster │ Release=19.24.0.0.0                      │ ║
║ │ [08:05:02] ALERTA ORA-01654: Tablespace ERP_DATA_TBS al 93.0% │ Bloqueo TX detectado en SID 512 -> SID 289       │ ║
║ └──────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘ ║
╚══════════════════════════════════════════════════════════════════════════════════════════════════════════════════════╝
```

---

## ⚙️ Decisiones Clave de Arquitectura

### 1. ¿Por qué `CGO_ENABLED=0` y cero librerías de cliente C?
La mayoría de los drivers de Oracle requieren `CGO` y enlazar dinámicamente contra `libclntsh.so` (Oracle Instant Client). En entornos empresariales con RHEL / Oracle Linux 7, 8 y 9 y distintas versiones de `glibc`, distribuir binarios con dependencias dinámicas es una fuente constante de problemas.

En `ora-tui`, el binario se compila 100% estático (`CGO_ENABLED=0 GOOS=linux GOARCH=amd64`) y se comunica directamente con las herramientas nativas que ya residen en `$ORACLE_HOME/bin` y `$GRID_HOME/bin` (`sqlplus -s / as sysdba`, `srvctl`, `crsctl`, `olsnodes`, `lsnrctl`, `rman`, `adrci`).

### 2. Auto-Descubrimiento sin Archivos de Configuración
Nada más ejecutarse, `ora-tui` descubre qué tipo de instalación Oracle tiene delante sin pedir un solo archivo `.yaml` o `.env`:

1. **Topología del Servidor:**
   Lee `/etc/oracle/ocr.loc` y `/etc/oracle/olr.loc`:
   * `local_only=false` $\rightarrow$ **ORACLE RAC**
   * `local_only=true` $\rightarrow$ **ORACLE RESTART (Standalone Grid)**
   * No existe $\rightarrow$ **SINGLE INSTANCE**
2. **Homes e Instancias Vivas en `/proc`:**
   Además de leer `/etc/oratab` e `inventory.xml`, escanea `/proc/[0-9]*/cmdline` para localizar procesos `ora_pmon_<SID>`, `db_pmon_<SID>` (Oracle 21c/23ai) y `asm_pmon_+ASM*`. Así detecta bases de datos levantadas aunque nadie las haya registrado en `/etc/oratab`.

```go
func ExtractSIDFromPMON(cmdline string) (sid string, isASM bool) {
	fields := strings.Fields(cmdline)
	if len(fields) == 0 {
		return "", false
	}
	proc := filepath.Base(fields[0])
	switch {
	case strings.HasPrefix(proc, "ora_pmon_"):
		return strings.TrimPrefix(proc, "ora_pmon_"), false
	case strings.HasPrefix(proc, "db_pmon_"):
		return strings.TrimPrefix(proc, "db_pmon_"), false
	case strings.HasPrefix(proc, "asm_pmon_"):
		return strings.TrimPrefix(proc, "asm_pmon_"), true
	}
	return "", false
}
```

### 3. Cambio Dinámico de Usuario UNIX (`root` $\rightarrow$ `oracle` / `grid`)
En un cluster RAC con separación de roles, `grid` es dueño de `$GRID_HOME` y `oracle` es dueño de `$ORACLE_HOME`. Ejecutar `sqlplus` o `srvctl` como `root` falla o ensucia permisos.

`ora-tui` lee el `uid` y `gid` propietarios de cada directorio con `os.Stat` y lanza cada subproceso con las credenciales exactas de ese usuario mediante `syscall.Credential`, reservando `root` **únicamente** para `crsctl start/stop crs`:

```go
if !asRoot && targetOwner != nil && targetOwner.UID != 0 && currentUID == 0 {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{
			Uid: targetOwner.UID,
			Gid: targetOwner.GID,
		},
	}
}
```

### 4. Marcos Simples (`┌─┐`) y Dobles (`╔═╗`) en `tview`
La librería [`rivo/tview`](https://github.com/rivo/tview) permite definir globalmente los runas de dibujo de cajas, distinguiendo entre ventanas normales y ventanas con foco activo. Configurando `tview.Borders`, conseguimos que cualquier tabla o menú emergente resalte automáticamente con borde doble cuando el usuario lo selecciona con las flechas:

```go
// Borde simple (┌ ─ ┐ │ └ ┘) para contenedores y tablas estándar
tview.Borders.Horizontal = '─'
tview.Borders.Vertical = '│'
tview.Borders.TopLeft = '┌'
tview.Borders.TopRight = '┐'
tview.Borders.BottomLeft = '└'
tview.Borders.BottomRight = '┘'

// Borde doble (╔ ═ ╗ ║ ╚ ╝) para la ventana con foco y modales emergentes
tview.Borders.HorizontalFocus = '═'
tview.Borders.VerticalFocus = '║'
tview.Borders.TopLeftFocus = '╔'
tview.Borders.TopRightFocus = '╗'
tview.Borders.BottomLeftFocus = '╚'
tview.Borders.BottomRightFocus = '╝'
```

Además, pulsando la tecla **`b`** en cualquier momento, puedes alternar en caliente entre el modo mixto (`┌─┐` + `╔═╗`), todo borde doble (`╔═╗`), todo borde simple (`┌─┐`) o ASCII clásico de 7 bits (`+ - = |`) si entras desde una consola serie antigua.

---

## 🛠️ Los 8 Módulos de Diagnóstico y Mantenimiento Guiado

En lugar de memorizar comandos, te mueves con las **flechas del teclado (`↑`/`↓`)** por las 8 vistas y pulsas **`Enter`** sobre cualquier fila para abrir su menú contextual de mantenimiento:

1. **Cluster & DB Estado:** Salud de demonios (`CRS`, `CSSD`, `EVMD`, `ASM`), VIPs, instancias, PDBs y **Drenado Seguro de Nodo (Graceful Node Drain)** en 4 pasos:
   * *Paso 1:* Pre-chequeo de nodos failover `ONLINE`.
   * *Paso 2:* Reubicación en caliente de servicios (`srvctl relocate service`).
   * *Paso 3:* Parada limpia de la instancia local (`srvctl stop instance -stopoption IMMEDIATE`).
   * *Paso 4:* Parada opcional de la pila de clusterware (`crsctl stop crs`).
2. **Usuarios Conectados (`gv$session`):** Detección instantánea de bloqueos `TX/TM`, `SQL_ID`, evento de espera y texto SQL actual, con opción para ejecutar `KILL SESSION IMMEDIATE` o `DISCONNECT SESSION POST_TRANSACTION`.
3. **Tablespaces & Datafiles:** Barras visuales de llenado y asistente para añadir datafiles en ASM (`+DATA`), activar `AUTOEXTEND ON` o hacer `COALESCE`.
4. **Tablas, Segmentos & Objetos Inválidos:** Identificación de tablas con estadísticas `STALE` o alta fragmentación para lanzar `DBMS_STATS.GATHER_TABLE_STATS`, `SHRINK SPACE CASCADE`, `REBUILD ONLINE` de índices o recompilar objetos inválidos con `UTL_RECOMP`.
5. **ASM & Red Listeners:** Monitorización de diskgroups (`v$asm_diskgroup`) y control de listeners locales y SCAN (`lsnrctl start/stop/reload`).
6. **RMAN, Flashback & Data Guard:** Estado de backups RMAN, creación de *Guaranteed Restore Points (GRP)* antes de un cambio crítico y verificación de lag en réplicas *Data Guard Standby*.
7. **Alert Log & Errores `ORA-`:** Lectura directa sobre `v$diag_alert_ext` con salto automático al módulo que resuelve el error y purgado de trazas con `adrci`.
8. **Centro de Mantenimiento:** Catálogo guiado de operaciones DBA con confirmación de seguridad y consola de auditoría en tiempo real.

---

## 🚀 Pruébalo en 30 Segundos (Modo Simulación `--mock`)

No necesitas tener un cluster Oracle RAC instalado en tu máquina para probar la interfaz. `ora-tui` incluye un modo simulador (`--mock`) con datos realistas de un cluster RAC en producción (incluyendo sesiones bloqueadas, un tablespace al 93% y alertas `ORA-01654`):

```bash
# Instalar directamente en RHEL / Oracle Linux 7, 8 o 9:
sudo dnf install -y https://github.com/mario-ezquerro/oratui/raw/main/dist/ora-tui-1.0.0-1.x86_64.rpm
sudo ora-tui

# O clonar y probar el simulador interactivo en cualquier Mac/Linux:
git clone https://github.com/mario-ezquerro/oratui.git
cd oratui
make mock
```

¿Qué otra tarea repetitiva de administración Oracle te gustaría automatizar desde un menú de consola? ¡Te leo en los comentarios!
