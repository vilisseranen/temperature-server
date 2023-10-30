import time
import network
import json
import ntptime
from machine import I2C, Pin

from am2320 import AM2320
from sht30 import SHT30

from umqtt.simple2 import MQTTClient

# supported devices: (<i2c address>, <name>)
sensor_SHT30 = (68, "SHT30")
sensor_AM2320 = (92, "AM2320")

T_OFFSET = 0  # T_OFFSET will be added to the temperature measure
H_OFFSET = 0  # H_OFFSET will be added to the humidity measure

i2c = I2C(0, sda=Pin(4, Pin.PULL_UP), scl=Pin(5, Pin.PULL_UP), freq=100000)

print("Detecting sensor")
device_id = []
while device_id == [] or (
    len(device_id) > 0 and device_id[0] not in [sensor_SHT30[0], sensor_AM2320[0]]
):
    print("device_id: {}".format(device_id))
    time.sleep(0.1)
    device_id = i2c.scan()
print(device_id)
device_id = device_id[0]

if device_id == sensor_SHT30[0]:
    device = sensor_SHT30
    sensor = SHT30(0, sda_pin=4, scl_pin=5, i2c_address=68)
elif device_id == sensor_AM2320[0]:
    device = sensor_AM2320
    sensor = AM2320(i2c=i2c)
else:
    raise Exception("sensor not supported")
print("Using sensor {}".format(device[1]))
time.sleep(1)

# Will set the source for the data points
with open("source.config") as f:
    SOURCE = f.read()

# Get offsets (optional)
# format is <T_OFFSET>,<H_OFFSET>
try:
    with open("offsets.config") as f:
        offsets = f.read()
        T_OFFSET, H_OFFSET = offsets.split(",")
        T_OFFSET = float(T_OFFSET)
        H_OFFSET = float(H_OFFSET)
except:
    print("No offsets defined")
    pass
print("Using T_OFFSET={} and H_OFFSET={}".format(T_OFFSET, H_OFFSET))

# setup wifi
# read config and set as AP
with open("wifi.config") as f:
    config = f.read()
ssid, password = config.split(",")

station = network.WLAN(network.STA_IF)
station.active(True)
print("station is connected: {}".format(station.isconnected()))
print("station ifconfig: {}".format(station.ifconfig()))

while True:
    # If not connected, try to connect
    if not station.isconnected():
        station.connect(ssid, password)
        time.sleep(10)
        print("station is connected: {}".format(station.isconnected()))
        print("station ifconfig: {}".format(station.ifconfig()))
        pass
    else:  # We should be connected
        # first set time
        try:
            print("Setting time")
            ntptime.timeout = 10
            ntptime.settime()
            print("Time set")
        except Exception as ex:
            print("time out while syncing time")
            pass
        # try to read the sensors and send the messages
        if device == sensor_AM2320:
            try:
                sensor.measure()
                temperature = sensor.temperature() + T_OFFSET
                humidity = sensor.humidity() + H_OFFSET
                print(
                    "Temperature {:>6}: {:.1f}ºC, RH: {:.1f}%".format(
                        "AM2320", temperature, humidity
                    )
                )
                payloadT = {
                    "metric": "temperature",
                    "timestamp": int(time.time()),
                    "value": temperature,
                    "tags": {"source": SOURCE, "sensor": device[1]},
                }
                payloadH = {
                    "metric": "humidity",
                    "timestamp": int(time.time()),
                    "value": humidity,
                    "tags": {"source": SOURCE, "sensor": device[1]},
                }
                print(json.dumps(payloadT))
                print(json.dumps(payloadH))
                c = MQTTClient("umqtt_client", "pi.hole")
                c.connect()
                c.publish(b"sensors", json.dumps(payloadT))
                c.publish(b"sensors", json.dumps(payloadH))
                c.disconnect()
            except Exception as ex:
                print("Error in AM2320:", ex)
        elif device == sensor_SHT30:
            try:
                temperature, humidity = sensor.measure()
                temperature = temperature + T_OFFSET
                humidity = humidity + H_OFFSET
                print(
                    "Temperature {:>6}: {:.1f}ºC, RH: {:.1f}%".format(
                        "SHT30", temperature, humidity
                    )
                )
                payloadT = {
                    "metric": "temperature",
                    "timestamp": int(time.time()),
                    "value": temperature,
                    "tags": {"source": SOURCE, "sensor": device[1]},
                }
                payloadH = {
                    "metric": "humidity",
                    "timestamp": int(time.time()),
                    "value": humidity,
                    "tags": {"source": SOURCE, "sensor": device[1]},
                }
                print(json.dumps(payloadT))
                print(json.dumps(payloadH))
                c = MQTTClient("umqtt_client", "pi.hole")
                c.connect()
                c.publish(b"sensors", json.dumps(payloadT))
                c.publish(b"sensors", json.dumps(payloadH))
                c.disconnect()

            except Exception as ex:
                print("Error in SHT30:", ex)

        # listen for config changes
        time.sleep(60)
