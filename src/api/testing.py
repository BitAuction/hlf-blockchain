from aux import map_transaction_id
import random


randomlist=[1,2,3]
transactions=["d0c513ae211c64180723a00b0c9c370b2d57495965306b0175abd9b4f9a7820d",'97c8c61cb1f9ab17c583b6b25efc9b713f64af55acf006b65ad87b39374ba128']

for i in transactions:
    mapped_value=map_transaction_id(i)
    random.seed(mapped_value)
    random.shuffle(randomlist)
    print(mapped_value)



