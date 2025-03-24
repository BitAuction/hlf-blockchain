import zlib

def map_transaction_id(transaction_id):
    # Calculate CRC32 hash of the transaction ID
    crc32_hash = zlib.crc32(transaction_id.encode())

    # Map the hash value to a number between 1 and 10
    mapped_value = (crc32_hash % 10) + 1

    return mapped_value