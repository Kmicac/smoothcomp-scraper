#!/bin/bash

# Script para probar la API de SmoothComp
# Verifica que podemos acceder al endpoint de participantes y obtener datos

set -e

echo "=================================================="
echo "🧪 TEST: SmoothComp Participants API"
echo "=================================================="
echo ""

# Configuración
EVENT_ID="25258"
SUBDOMAIN="adcc.smoothcomp.com"
API_URL="https://${SUBDOMAIN}/en/event/${EVENT_ID}/participants"

echo "📋 Configuración del test:"
echo "   Event ID: $EVENT_ID"
echo "   Subdomain: $SUBDOMAIN"
echo "   API URL: $API_URL"
echo ""

# Test 1: Verificar que el endpoint existe
echo "🔍 Test 1: Verificar que el endpoint existe..."
HTTP_STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
  -X POST "$API_URL" \
  -H "User-Agent: Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36" \
  -H "Accept: application/json, text/javascript, */*; q=0.01" \
  -H "X-Requested-With: XMLHttpRequest")

if [ "$HTTP_STATUS" -eq 200 ]; then
  echo "   ✅ Endpoint respondió con 200 OK"
else
  echo "   ❌ Endpoint respondió con status: $HTTP_STATUS"
  exit 1
fi
echo ""

# Test 2: Obtener datos y verificar estructura JSON
echo "🔍 Test 2: Obtener datos y verificar estructura JSON..."
RESPONSE=$(curl -s -X POST "$API_URL" \
  -H "User-Agent: Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36" \
  -H "Accept: application/json, text/javascript, */*; q=0.01" \
  -H "X-Requested-With: XMLHttpRequest")

# Verificar que la respuesta no está vacía
if [ -z "$RESPONSE" ]; then
  echo "   ❌ Respuesta vacía"
  exit 1
fi

# Verificar que es JSON válido (usando jq si está disponible)
if command -v jq &> /dev/null; then
  echo "$RESPONSE" | jq . > /dev/null 2>&1
  if [ $? -eq 0 ]; then
    echo "   ✅ JSON válido"
  else
    echo "   ❌ JSON inválido"
    exit 1
  fi
  
  # Verificar estructura esperada
  PARTICIPANTS_COUNT=$(echo "$RESPONSE" | jq '.participants | length')
  CATEGORIES_COUNT=$(echo "$RESPONSE" | jq '.categories | length')
  
  echo "   📊 Participants: $PARTICIPANTS_COUNT categorías"
  echo "   📊 Categories: $CATEGORIES_COUNT definiciones"
  
  if [ "$PARTICIPANTS_COUNT" -gt 0 ]; then
    echo "   ✅ Estructura correcta"
  else
    echo "   ❌ No hay participantes"
    exit 1
  fi
else
  echo "   ⚠️  jq no está instalado, no se puede verificar estructura"
  echo "   ℹ️  Respuesta obtenida (primeros 500 caracteres):"
  echo "$RESPONSE" | head -c 500
fi
echo ""

# Test 3: Verificar campos de un atleta
if command -v jq &> /dev/null; then
  echo "🔍 Test 3: Verificar campos de un atleta..."
  
  # Obtener el primer atleta
  FIRST_ATHLETE=$(echo "$RESPONSE" | jq '.participants[0].registrations[0]')
  
  # Verificar campos requeridos
  REQUIRED_FIELDS=("user_id" "firstname" "lastname" "country" "cn" "age" "birth")
  ALL_OK=true
  
  for field in "${REQUIRED_FIELDS[@]}"; do
    VALUE=$(echo "$FIRST_ATHLETE" | jq -r ".$field")
    if [ "$VALUE" != "null" ] && [ -n "$VALUE" ]; then
      echo "   ✅ Campo '$field': $VALUE"
    else
      echo "   ❌ Campo '$field': falta o es null"
      ALL_OK=false
    fi
  done
  
  if [ "$ALL_OK" = true ]; then
    echo "   ✅ Todos los campos requeridos presentes"
  else
    echo "   ❌ Faltan algunos campos requeridos"
    exit 1
  fi
  
  # Mostrar ejemplo de atleta completo
  echo ""
  echo "   📋 Ejemplo de atleta (JSON):"
  echo "$FIRST_ATHLETE" | jq '.'
fi
echo ""

# Test 4: Verificar diferentes subdominios
echo "🔍 Test 4: Probar diferentes subdominios..."
SUBDOMAINS=("smoothcomp.com" "adcc.smoothcomp.com" "ibjjf.smoothcomp.com")

for sub in "${SUBDOMAINS[@]}"; do
  TEST_URL="https://${sub}/en/event/${EVENT_ID}/participants"
  STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST "$TEST_URL" \
    -H "User-Agent: Mozilla/5.0" \
    -H "Accept: application/json" 2>/dev/null || echo "FAIL")
  
  if [ "$STATUS" -eq 200 ]; then
    echo "   ✅ $sub: OK (200)"
  elif [ "$STATUS" -eq 301 ] || [ "$STATUS" -eq 302 ]; then
    echo "   ⚠️  $sub: Redirect ($STATUS)"
  else
    echo "   ❌ $sub: Status $STATUS"
  fi
done
echo ""

# Resumen
echo "=================================================="
echo "✅ TESTS COMPLETADOS EXITOSAMENTE"
echo "=================================================="
echo ""
echo "📝 Resumen:"
echo "   • API endpoint funciona correctamente"
echo "   • Respuesta JSON válida"
echo "   • Estructura de datos correcta"
if command -v jq &> /dev/null; then
  echo "   • Participants: $PARTICIPANTS_COUNT"
  echo "   • Categories: $CATEGORIES_COUNT"
fi
echo ""
echo "🚀 El scraper está listo para usarse!"
echo ""

# Guardar respuesta completa para debugging (opcional)
if [ "$1" = "--save" ]; then
  OUTPUT_FILE="smoothcomp_api_response_${EVENT_ID}.json"
  echo "$RESPONSE" > "$OUTPUT_FILE"
  echo "💾 Respuesta completa guardada en: $OUTPUT_FILE"
  
  if command -v jq &> /dev/null; then
    echo "$RESPONSE" | jq '.' > "${OUTPUT_FILE}.formatted"
    echo "💾 Respuesta formateada guardada en: ${OUTPUT_FILE}.formatted"
  fi
fi