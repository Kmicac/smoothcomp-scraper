# Smoothcomp Scraper

Scraper backend para obtener informacion publica de Smoothcomp y guardarla en una base local.

## Tecnologia
- Go (backend y scraping)
- SQLite (almacenamiento local)
- GORM (ORM)
- Gorilla Mux (API HTTP)
- Colly y Goquery (scraping y parsing HTML)

## Que hace este scraper
El servicio expone endpoints HTTP para disparar scraping y consultar datos guardados.
La informacion se obtiene desde paginas publicas de Smoothcomp, endpoints JSON de Smoothcomp
y perfiles de atletas.

## Informacion que trae hoy

### Academias
- Nombre, pais, codigo de pais, logo, website y redes sociales (si existen)
- Estadisticas (wins/losses, medallas)

### Atletas (listado por evento)
- Identidad basica (nombre, pais, genero, edad)
- Perfil y avatar
- Vinculo con academia si esta disponible

### Perfiles de atletas (enrichment)
- Cinturon, afiliacion, imagen
- Estadisticas de wins/losses y desglose por tipo

### Eventos (listado)
- Nombre, URL, imagen
- Ciudad, pais, codigo de pais
- Fecha (texto) y estado (dias restantes si aplica)
- Tipo (past/upcoming) y seccion

### Detalle de evento
- Nombre, descripcion, fechas, imagen
- Ubicacion y organizador
- Bloques de informacion extendida (info panels y CMS blocks) en JSON

## Base de datos
Por defecto se usa SQLite en `./storage/cache.db` (configurable con `CACHE_DB_PATH` en `.env`).

